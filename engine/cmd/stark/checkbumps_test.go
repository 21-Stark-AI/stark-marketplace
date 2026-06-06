package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// On a clean repo (committed index matches the catalog with no un-bumped source
// edits), check-bumps must exit 0.
func TestCheckBumpsCleanRepoExitsZero(t *testing.T) {
	root := repoRoot(t)
	// ensure committed output is current so the previous index is coherent
	if code := runBuild(filepath.Join(root, "catalog"), root, false); code != 0 {
		t.Fatalf("pre-build want 0, got %d", code)
	}
	if code := runCheckBumps(filepath.Join(root, "catalog"), root); code != 0 {
		t.Fatalf("clean check-bumps want 0, got %d", code)
	}
}

// Exercises the full CLI seam (git show → leanPrev parse → digest recompute →
// keying → exit 1): a previous index.json committed with a STALE digest but the
// SAME version as the current source is a version-bump-gate violation.
func TestCheckBumpsDetectsViolation(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	cmdDir := filepath.Join(root, "catalog", "demo", "commands")
	must(os.MkdirAll(cmdDir, 0o755))
	must(os.WriteFile(filepath.Join(root, "catalog", "demo", "bundle.yaml"),
		[]byte("name: demo\nversion: 0.1.0\ndescription: d\nowner: { name: E }\nruntimes: [claude]\n"), 0o644))
	must(os.WriteFile(filepath.Join(cmdDir, "hello.md"),
		[]byte("---\nname: hello\ntype: command\ndescription: d\nversion: 0.1.0\n---\nbody\n"), 0o644))
	// previous index.json: same version, stale digest → must trip the gate.
	must(os.WriteFile(filepath.Join(root, "index.json"),
		[]byte(`{"schemaVersion":1,"artifacts":[{"name":"hello","bundle":"demo","version":"0.1.0","digest":"sha256:stale"}]}`+"\n"), 0o644))

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("add", ".")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "seed")

	if code := runCheckBumps(filepath.Join(root, "catalog"), root); code != 1 {
		t.Fatalf("want exit 1 on un-bumped source change, got %d", code)
	}
}
