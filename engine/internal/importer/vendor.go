package importer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/21StarkCom/bifrost/engine/internal/model"
)

// vendorToolsSkipDirs are tool subdirectories never shipped to a plugin: tests,
// fixtures, and installed deps (node_modules carries .ts type stubs we must not
// vendor).
var vendorToolsSkipDirs = map[string]bool{
	"__tests__":    true,
	"fixtures":     true,
	"node_modules": true,
}

// skillSupportDirs are the conventional per-skill subtrees shipped alongside
// the rendered SKILL.md. Keep this list explicit and ordered so snapshot
// construction remains deterministic as new support-directory types are added.
var skillSupportDirs = [...]string{
	"references",
	"scripts",
	"assets",
}

// alwaysSkipDirs are junk/VCS/build-cache dirs pruned from EVERY vendored tree
// (including the verbatim scripts/standards/prompts copies), regardless of the
// per-call skipDirs. Prevents e.g. stray Python __pycache__/ from a source
// checkout leaking .pyc into the committed snapshot.
var alwaysSkipDirs = map[string]bool{
	"__pycache__": true,
	".git":        true,
	".remember":   true,
}

// alwaysSkipFile reports junk files pruned from every vendored tree.
func alwaysSkipFile(rel string) bool {
	base := filepath.Base(rel)
	return strings.HasSuffix(base, ".pyc") || base == ".DS_Store"
}

// VendorSnapshot reads a stark-skills checkout and returns the normalized
// immutable-asset snapshot (vendor-relative path -> content) that `stark build`
// vendors verbatim into every plugin. Layout:
//
//	tools/<f>.ts           <- <from>/tools  (.ts only; excl *.test.ts, __tests__/, fixtures/, node_modules/)
//	prompts/**             <- <from>/global/prompts
//	standards/**           <- <from>/standards
//	scripts/**             <- <from>/scripts
//	data/persona/**        <- <from>/data/persona
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
		{filepath.Join(from, "data", "persona"), "data/persona"},
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

// RuntimeOverrideSupportSnapshot captures the runtime-specific support files
// needed by one bundle. Returned keys use plugin-relative layout, ready to be
// written below vendor/runtime-overrides/<runtime>/<bundle>/:
//
//	global/config.json -> config.json
//	standards/**       -> standards/**
//	tools/**           -> tools/**
//	plugins/<bundle>/tools/** -> tools/**
//	skill/<member>/{references,scripts,assets}/**
//	                    -> skills/<member>/{references,scripts,assets}/**
//
// Artifact Markdown is imported into catalog overrides and is never returned
// here. The resulting tree is separate from the shared/per-plugin snapshots
// used by Claude.
func RuntimeOverrideSupportSnapshot(from string, runtime model.Runtime, bundle string, skills []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	root := filepath.Join(from, "runtime-overrides", string(runtime))
	if fi, err := os.Stat(root); os.IsNotExist(err) {
		return out, nil
	} else if err != nil {
		return nil, fmt.Errorf("runtime override %s: %w", runtime, err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("runtime override %s: %s is not a directory", runtime, root)
	}

	config, err := os.ReadFile(filepath.Join(root, "global", "config.json"))
	if err == nil {
		out["config.json"] = config
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("runtime override %s config: %w", runtime, err)
	}

	for _, tree := range []struct{ src, dst string }{
		{src: filepath.Join(root, "standards"), dst: "standards"},
		{src: filepath.Join(root, "tools"), dst: "tools"},
		// Bundle-specific runtime tools override only their owning plugin. This
		// keeps command helpers such as stark-gh cleanup out of unrelated Codex
		// packages while preserving the same final tools/ layout.
		{src: filepath.Join(root, "plugins", bundle, "tools"), dst: "tools"},
	} {
		if fi, err := os.Stat(tree.src); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("runtime override %s %s: %w", runtime, tree.dst, err)
		} else if !fi.IsDir() {
			return nil, fmt.Errorf("runtime override %s: %s is not a directory", runtime, tree.src)
		}
		// Runtime overlays are explicitly curated source, so copy these trees
		// verbatim instead of applying the broad shared-vendor tools filter.
		if err := copyTree(tree.src, tree.dst, out, nil, nil); err != nil {
			return nil, fmt.Errorf("runtime override %s %s: %w", runtime, tree.dst, err)
		}
	}

	for _, skill := range skills {
		for _, supportDir := range skillSupportDirs {
			src := filepath.Join(root, "skill", skill, supportDir)
			if fi, err := os.Stat(src); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return nil, fmt.Errorf("runtime override %s skill %s/%s: %w", runtime, skill, supportDir, err)
			} else if !fi.IsDir() {
				return nil, fmt.Errorf("runtime override %s: %s is not a directory", runtime, src)
			}
			dst := "skills/" + skill + "/" + supportDir
			if err := copyTree(src, dst, out, nil, nil); err != nil {
				return nil, fmt.Errorf("runtime override %s skill %s/%s: %w", runtime, skill, supportDir, err)
			}
		}
	}
	return out, nil
}

// PluginVendorSnapshot reads a stark-skills checkout and returns the per-bundle
// asset snapshot (bundle-relative path -> content), which `stark build` layers
// into THAT bundle's dist tree only (winning over the shared VendorSnapshot). It
// captures per-bundle files the shared snapshot does not provide and the adapter
// does not render:
//
//	skills/<name>/{references,scripts,assets}/** <- matching subtrees under skill/<name>/
//	tools/<f>.ts                              <- plugins/<bundle>/tools  (.ts only; excl *.test.ts, __tests__/, fixtures/, node_modules/)
//	config.json                               <- plugins/<bundle>/config.json   (the plugin's OWN config, e.g. stark-gh's {draft};
//	                                             overrides the shared global config.json for this bundle)
//	package.json                              <- plugins/<bundle>/package.json   (e.g. {"type":"module"} — pins ESM resolution so the
//	                                             vendored .ts tools run under `node --experimental-strip-types` regardless of ancestor dirs)
//
// `skills` is the bundle's membership list; each skill's conventional supporting
// subtrees are shipped so a marketplace-installed skill carries the files its
// SKILL.md points to (the adapter renders only SKILL.md — support files would
// otherwise be dropped). Returns an empty (non-nil) map when the bundle has no
// plugin dir AND none of its skills have support directories, which is not an error.
// commands/ + mcp/ are NOT captured here; they are imported as artifacts by
// importPlugin and rendered by the adapter.
func PluginVendorSnapshot(from, bundle string, skills []string) (map[string][]byte, error) {
	out := map[string][]byte{}

	// Per-skill support files retain their subtree beside the rendered skill.
	for _, name := range skills {
		for _, supportDir := range skillSupportDirs {
			srcDir := filepath.Join(from, "skill", name, supportDir)
			if fi, err := os.Stat(srcDir); err != nil || !fi.IsDir() {
				continue // most skills omit one or more support directories
			}
			if err := copyTree(srcDir, "skills/"+name+"/"+supportDir, out, nil, nil); err != nil {
				return nil, fmt.Errorf("plugin vendor skill %s (%s/%s): %w", supportDir, bundle, name, err)
			}
		}
	}

	// Per-bundle plugin assets (plugins/<bundle>/): tools + the plugin's own seed files.
	pluginDir := filepath.Join(from, "plugins", bundle)
	if fi, err := os.Stat(pluginDir); err == nil && fi.IsDir() {
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
			if p != src && (alwaysSkipDirs[d.Name()] || (skipDirs != nil && skipDirs[d.Name()])) {
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
		if alwaysSkipFile(rel) {
			return nil
		}
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
