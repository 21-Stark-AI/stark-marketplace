package importer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// vendorToolsSkipDirs are tool subdirectories never shipped to a plugin: tests,
// fixtures, and installed deps (node_modules carries .ts type stubs we must not
// vendor).
var vendorToolsSkipDirs = map[string]bool{
	"__tests__":    true,
	"fixtures":     true,
	"node_modules": true,
}

// VendorSnapshot reads a stark-skills checkout and returns the normalized
// immutable-asset snapshot (vendor-relative path -> content) that `stark build`
// vendors verbatim into every plugin. Layout:
//
//	tools/<f>.ts           <- <from>/tools  (.ts only; excl *.test.ts, __tests__/, fixtures/, node_modules/)
//	prompts/**             <- <from>/global/prompts
//	standards/**           <- <from>/standards
//	scripts/**             <- <from>/scripts
//	config.json            <- <from>/global/config.json
//	forge_heuristics.json  <- <from>/global/forge_heuristics.json
func VendorSnapshot(from string) (map[string][]byte, error) {
	out := map[string][]byte{}

	// tools: runtime .ts only.
	if err := copyTree(filepath.Join(from, "tools"), "tools", out, vendorToolsSkipDirs, func(rel string) bool {
		return strings.HasSuffix(rel, ".ts") && !strings.HasSuffix(rel, ".test.ts")
	}); err != nil {
		return nil, fmt.Errorf("vendor tools: %w", err)
	}

	// whole trees, copied verbatim.
	for _, m := range []struct{ src, dst string }{
		{filepath.Join(from, "global", "prompts"), "prompts"},
		{filepath.Join(from, "standards"), "standards"},
		{filepath.Join(from, "scripts"), "scripts"},
	} {
		if err := copyTree(m.src, m.dst, out, nil, nil); err != nil {
			return nil, fmt.Errorf("vendor %s: %w", m.dst, err)
		}
	}

	// single seed files.
	for _, f := range []struct{ src, dst string }{
		{filepath.Join(from, "global", "config.json"), "config.json"},
		{filepath.Join(from, "global", "forge_heuristics.json"), "forge_heuristics.json"},
	} {
		b, err := os.ReadFile(f.src)
		if err != nil {
			return nil, fmt.Errorf("vendor %s: %w", f.dst, err)
		}
		out[f.dst] = b
	}
	return out, nil
}

// PluginVendorSnapshot reads a stark-skills checkout and returns the per-bundle
// plugin asset snapshot (bundle-relative path -> content) for plugins/<bundle>,
// which `stark build` layers into THAT bundle's dist tree only (winning over the
// shared VendorSnapshot). It captures the plugin's own runtime files that the
// shared snapshot does not provide and the adapter does not render:
//
//	tools/<f>.ts   <- plugins/<bundle>/tools  (.ts only; excl *.test.ts, __tests__/, fixtures/, node_modules/)
//	config.json    <- plugins/<bundle>/config.json   (the plugin's OWN config, e.g. stark-gh's {draft};
//	                  overrides the shared global config.json for this bundle)
//	package.json   <- plugins/<bundle>/package.json   (e.g. {"type":"module"} — pins ESM resolution so the
//	                  vendored .ts tools run under `node --experimental-strip-types` regardless of ancestor dirs)
//
// Returns an empty (non-nil) map when plugins/<bundle> is absent — a skills-only
// bundle has no plugin assets, which is not an error. commands/ + mcp/ are NOT
// captured here; they are imported as artifacts by importPlugin and rendered by
// the adapter.
func PluginVendorSnapshot(from, bundle string) (map[string][]byte, error) {
	out := map[string][]byte{}
	pluginDir := filepath.Join(from, "plugins", bundle)
	if fi, err := os.Stat(pluginDir); err != nil || !fi.IsDir() {
		return out, nil // absent plugin dir = no per-bundle assets (skills-only bundle)
	}

	// tools: runtime .ts only (same filter as the shared snapshot).
	toolsDir := filepath.Join(pluginDir, "tools")
	if fi, err := os.Stat(toolsDir); err == nil && fi.IsDir() {
		if err := copyTree(toolsDir, "tools", out, vendorToolsSkipDirs, func(rel string) bool {
			return strings.HasSuffix(rel, ".ts") && !strings.HasSuffix(rel, ".test.ts")
		}); err != nil {
			return nil, fmt.Errorf("plugin vendor tools (%s): %w", bundle, err)
		}
	}

	// single seed files, when present.
	for _, name := range []string{"config.json", "package.json"} {
		b, err := os.ReadFile(filepath.Join(pluginDir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("plugin vendor %s (%s): %w", name, bundle, err)
		}
		out[name] = b
	}
	return out, nil
}

// copyTree walks src and records each regular file into out under
// "<dstPrefix>/<rel>". Directories whose base name is in skipDirs are pruned.
// When keep is non-nil, only relative paths for which keep returns true are
// included.
func copyTree(src, dstPrefix string, out map[string][]byte, skipDirs map[string]bool, keep func(rel string) bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs != nil && skipDirs[d.Name()] && p != src {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if keep != nil && !keep(rel) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[dstPrefix+"/"+rel] = b
		return nil
	})
}
