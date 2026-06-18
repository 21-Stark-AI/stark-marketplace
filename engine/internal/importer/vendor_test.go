package importer

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a tiny test helper: mkdir -p the parent and write content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPluginVendorSnapshot verifies the per-bundle plugin snapshot captures the
// plugin's runtime .ts tools (flat + lib/) plus its own config.json/package.json,
// applies the same filtering as the shared snapshot (drop *.test.ts, __tests__/,
// node_modules/, and non-.ts files), and keys everything bundle-relative.
func TestPluginVendorSnapshot(t *testing.T) {
	from := t.TempDir()
	gh := filepath.Join(from, "plugins", "stark-gh")
	writeFile(t, filepath.Join(gh, "tools", "gh_cleanup.ts"), "export const x = 1\n")
	writeFile(t, filepath.Join(gh, "tools", "lib", "git.ts"), "export const g = 1\n")
	writeFile(t, filepath.Join(gh, "tools", "lib", "draft_schema.json"), `{"not":"vendored"}`) // non-.ts: dropped
	writeFile(t, filepath.Join(gh, "tools", "gh_cleanup.test.ts"), "test")                     // *.test.ts: dropped
	writeFile(t, filepath.Join(gh, "tools", "__tests__", "x.ts"), "test")                      // __tests__/: dropped
	writeFile(t, filepath.Join(gh, "tools", "node_modules", "dep", "y.ts"), "dep")             // node_modules/: dropped
	writeFile(t, filepath.Join(gh, "config.json"), `{"draft":{}}`)
	writeFile(t, filepath.Join(gh, "package.json"), `{"type":"module"}`)
	writeFile(t, filepath.Join(gh, "README.md"), "# not vendored")

	got, err := PluginVendorSnapshot(from, "stark-gh")
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"tools/gh_cleanup.ts": "export const x = 1\n",
		"tools/lib/git.ts":    "export const g = 1\n",
		"config.json":         `{"draft":{}}`,
		"package.json":        `{"type":"module"}`,
	}
	if len(got) != len(want) {
		t.Fatalf("snapshot has %d files, want %d: %v", len(got), len(want), keysOf(got))
	}
	for rel, content := range want {
		b, ok := got[rel]
		if !ok {
			t.Fatalf("missing %q; got %v", rel, keysOf(got))
		}
		if string(b) != content {
			t.Fatalf("%q = %q, want %q", rel, b, content)
		}
	}
	for _, drop := range []string{
		"tools/lib/draft_schema.json", "tools/gh_cleanup.test.ts",
		"tools/__tests__/x.ts", "tools/node_modules/dep/y.ts", "README.md",
	} {
		if _, ok := got[drop]; ok {
			t.Errorf("unexpectedly vendored %q", drop)
		}
	}
}

// TestPluginVendorSnapshotSkillsOnly verifies a bundle with no plugins/<bundle>
// dir (a skills-only bundle) yields an empty, non-nil snapshot — not an error.
func TestPluginVendorSnapshotSkillsOnly(t *testing.T) {
	got, err := PluginVendorSnapshot(t.TempDir(), "stark-analyze")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("snapshot is nil; want empty non-nil map")
	}
	if len(got) != 0 {
		t.Fatalf("snapshot non-empty for skills-only bundle: %v", keysOf(got))
	}
}

// TestPluginVendorSnapshotConfigOnly verifies a plugin with config.json but no
// tools/ still captures the config (e.g. a command-only plugin carrying state).
func TestPluginVendorSnapshotConfigOnly(t *testing.T) {
	from := t.TempDir()
	writeFile(t, filepath.Join(from, "plugins", "p", "config.json"), `{"k":1}`)
	got, err := PluginVendorSnapshot(from, "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || string(got["config.json"]) != `{"k":1}` {
		t.Fatalf("got %v, want only config.json", got)
	}
}

func keysOf(m map[string][]byte) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
