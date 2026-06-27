package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/digest"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// On a clean repo (committed index matches the catalog with no un-bumped source
// edits), check-bumps must exit 0.
func TestCheckBumpsCleanRepoExitsZero(t *testing.T) {
	root := tempRepoRoot(t)
	// ensure committed output is current so the previous index is coherent
	if code := runBuild(filepath.Join(root, "catalog"), root, "", "", false); code != 0 {
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
	// previous index.json: same version, stale digest → must trip the gate. The real index
	// always carries `type` (it is part of the per-artifact identity / bump key), so the fixture does too.
	must(os.WriteFile(filepath.Join(root, "index.json"),
		[]byte(`{"schemaVersion":1,"artifacts":[{"name":"hello","type":"command","bundle":"demo","version":"0.1.0","digest":"sha256:stale"}]}`+"\n"), 0o644))

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

// CC-5 keying: two artifacts with the SAME name but different types in one bundle must be
// gated independently. Here command/x has a stale digest (same version → violation) while
// agent/x is current (clean). Keying by bundle/name alone would collapse them and silently
// drop the command/x change; keying by bundle/type/name catches it (exit 1).
func TestCheckBumpsKeysByArtifactType(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	mkAll := func(p string) { must(os.MkdirAll(p, 0o755)) }
	write := func(p, s string) { must(os.WriteFile(p, []byte(s), 0o644)) }

	mkAll(filepath.Join(root, "catalog", "demo", "commands"))
	mkAll(filepath.Join(root, "catalog", "demo", "agents"))
	write(filepath.Join(root, "catalog", "demo", "bundle.yaml"),
		"name: demo\nversion: 0.1.0\ndescription: d\nowner: { name: E }\nruntimes: [claude]\n")
	write(filepath.Join(root, "catalog", "demo", "commands", "x.md"),
		"---\nname: x\ntype: command\ndescription: cmd\nversion: 0.1.0\n---\nbody\n")
	write(filepath.Join(root, "catalog", "demo", "agents", "x.md"),
		"---\nname: x\ntype: agent\ndescription: agt\nversion: 0.1.0\n---\nbody\n")

	// Current digest of agent/x so its previous-index entry is genuinely clean.
	cat, err := load.Load(filepath.Join(root, "catalog"))
	must(err)
	var agentDigest string
	for _, b := range cat.Bundles {
		for _, a := range b.Artifacts {
			if a.Type == model.TypeAgent && a.Name == "x" {
				agentDigest = digest.Source(a)
			}
		}
	}
	if agentDigest == "" {
		t.Fatal("agent/x did not load")
	}

	prev := `{"schemaVersion":1,"artifacts":[` +
		`{"name":"x","type":"command","bundle":"demo","version":"0.1.0","digest":"sha256:stale"},` +
		`{"name":"x","type":"agent","bundle":"demo","version":"0.1.0","digest":"` + agentDigest + `"}` +
		`]}`
	write(filepath.Join(root, "index.json"), prev+"\n")

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
		t.Fatalf("same-name/different-type: stale command/x must trip the gate (exit 1), got %d", code)
	}
}

// The RunE closure derives repoRoot via filepath.Dir(filepath.Clean(catalogDir)); the runCheckBumps
// unit tests pass repoRoot explicitly and never exercise it. Drive the command with a trailing-slash
// arg against the real repo: a correctly-derived repoRoot yields a clean gate (exit 0 → nil).
func TestCheckBumpsCmdDerivesRepoRootFromCatalogArg(t *testing.T) {
	root := tempRepoRoot(t)
	if code := runBuild(filepath.Join(root, "catalog"), root, "", "", false); code != 0 {
		t.Fatalf("pre-build want 0, got %d", code)
	}
	cmd := newCheckBumpsCmd()
	cmd.SetArgs([]string{filepath.Join(root, "catalog") + "/"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check-bumps trailing-slash arg: repoRoot mis-derived (%v)", err)
	}
}
