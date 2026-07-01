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

	got, err := PluginVendorSnapshot(from, "stark-gh", nil)
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
	got, err := PluginVendorSnapshot(t.TempDir(), "stark-analyze", nil)
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
	got, err := PluginVendorSnapshot(from, "p", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || string(got["config.json"]) != `{"k":1}` {
		t.Fatalf("got %v, want only config.json", got)
	}
}

// TestPluginVendorSnapshotSkillReferences verifies each bundle skill's references/
// subtree is captured as skills/<name>/references/** (so a marketplace-installed
// skill ships the docs its SKILL.md points to), even for a skills-only bundle with
// no plugins/<bundle> dir. SKILL.md itself is NOT captured here — the adapter
// renders it.
func TestPluginVendorSnapshotSkillReferences(t *testing.T) {
	from := t.TempDir()
	rp := filepath.Join(from, "skill", "stark-refactor-plan")
	writeFile(t, filepath.Join(rp, "SKILL.md"), "---\nname: stark-refactor-plan\n---\nbody\n")
	writeFile(t, filepath.Join(rp, "references", "backlog-schema.md"), "schema\n")
	writeFile(t, filepath.Join(rp, "references", "sub", "nested.md"), "nested\n")
	// a peer skill with no references/ must contribute nothing.
	writeFile(t, filepath.Join(from, "skill", "stark-review", "SKILL.md"), "---\nname: stark-review\n---\nbody\n")

	got, err := PluginVendorSnapshot(from, "stark-analyze", []string{"stark-refactor-plan", "stark-review"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"skills/stark-refactor-plan/references/backlog-schema.md": "schema\n",
		"skills/stark-refactor-plan/references/sub/nested.md":     "nested\n",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d files, want %d: %v", len(got), len(want), keysOf(got))
	}
	for rel, content := range want {
		if string(got[rel]) != content {
			t.Fatalf("%q = %q, want %q", rel, got[rel], content)
		}
	}
	if _, ok := got["skills/stark-refactor-plan/SKILL.md"]; ok {
		t.Error("SKILL.md must be rendered by the adapter, not vendored")
	}
}

// TestVendorSnapshotSkipsJunk verifies the shared snapshot prunes VCS/build-cache
// junk from the verbatim trees (scripts/**): stray Python __pycache__/*.pyc from a
// source checkout must never leak into the committed vendor snapshot, while the
// real script files are still captured.
func TestVendorSnapshotSkipsJunk(t *testing.T) {
	from := t.TempDir()
	writeFile(t, filepath.Join(from, "scripts", "register_triggers.sh"), "#!/bin/bash\n")
	writeFile(t, filepath.Join(from, "scripts", "__pycache__", "session_id.cpython-313.pyc"), "junk")
	writeFile(t, filepath.Join(from, "scripts", "emit_queue.pyc"), "junk")
	writeFile(t, filepath.Join(from, "scripts", ".DS_Store"), "junk")
	// the other verbatim/tool trees VendorSnapshot walks must exist.
	writeFile(t, filepath.Join(from, "tools", "x.ts"), "export const x = 1\n")
	writeFile(t, filepath.Join(from, "global", "prompts", "p.md"), "prompt\n")
	writeFile(t, filepath.Join(from, "standards", "s.md"), "standard\n")
	// minimal seed files VendorSnapshot requires.
	writeFile(t, filepath.Join(from, "global", "config.json"), "{}")
	writeFile(t, filepath.Join(from, "global", "forge_heuristics.json"), "{}")

	got, err := VendorSnapshot(from)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["scripts/register_triggers.sh"]; !ok {
		t.Fatalf("real script missing; got %v", keysOf(got))
	}
	for _, junk := range []string{
		"scripts/__pycache__/session_id.cpython-313.pyc",
		"scripts/emit_queue.pyc",
		"scripts/.DS_Store",
	} {
		if _, ok := got[junk]; ok {
			t.Errorf("junk unexpectedly vendored: %q", junk)
		}
	}
}

func keysOf(m map[string][]byte) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
