package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
)

// TestPluginAssetsOverrideSharedSnapshot verifies that per-bundle plugin assets
// (Options.PluginAssetsRoot/<bundle>) are layered into THAT bundle's dist tree
// over the shared AssetsSource: the plugin's config.json wins over the shared one
// and its plugin-specific tools appear, while bundles without plugin assets keep
// the shared snapshot untouched. This is the regression guard for the missing
// stark-gh tools (gh_cleanup.ts et al.) — the shared snapshot alone never carried
// the plugin-specific tools, so /stark-gh:cleanup crashed with MODULE_NOT_FOUND.
func TestPluginAssetsOverrideSharedSnapshot(t *testing.T) {
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}

	// Shared snapshot: a global config + a shared tool, vendored into every bundle.
	shared := t.TempDir()
	mustWrite(t, filepath.Join(shared, "config.json"), `{"global":true}`)
	mustWrite(t, filepath.Join(shared, "tools", "shared.ts"), "export const s = 1\n")

	// Per-bundle plugin assets for stark-gh only: its own config + a plugin tool.
	plugins := t.TempDir()
	mustWrite(t, filepath.Join(plugins, "stark-gh", "config.json"), `{"draft":{}}`)
	mustWrite(t, filepath.Join(plugins, "stark-gh", "tools", "gh_cleanup.ts"), "export const c = 1\n")

	out, err := Build(cat, Options{AssetsSource: shared, PluginAssetsRoot: plugins})
	if err != nil {
		t.Fatal(err)
	}

	// stark-gh: plugin config overrides the shared one; both shared + plugin tools present.
	assertFile(t, out, "dist/claude/stark-gh/config.json", `{"draft":{}}`)
	assertFile(t, out, "dist/claude/stark-gh/tools/gh_cleanup.ts", "export const c = 1\n")
	assertFile(t, out, "dist/claude/stark-gh/tools/shared.ts", "export const s = 1\n")

	// A skills-only bundle keeps the shared snapshot: shared config, NO plugin override.
	assertFile(t, out, "dist/claude/stark-analyze/config.json", `{"global":true}`)
	if _, ok := out.Files["dist/claude/stark-analyze/tools/gh_cleanup.ts"]; ok {
		t.Error("stark-analyze unexpectedly got stark-gh's plugin tool")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, out Output, rel, want string) {
	t.Helper()
	b, ok := out.Files[rel]
	if !ok {
		t.Fatalf("missing %q", rel)
	}
	if string(b) != want {
		t.Fatalf("%q = %q, want %q", rel, b, want)
	}
}
