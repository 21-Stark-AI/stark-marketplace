package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/provenance"
)

func TestBuildCheckExitCodes(t *testing.T) {
	root := repoRoot(t)
	// write a fresh build, then --check must be clean (exit 0)
	if code := runBuild(filepath.Join(root, "catalog"), root, "", "", false); code != 0 {
		t.Fatalf("build write want 0, got %d", code)
	}
	if code := runBuild(filepath.Join(root, "catalog"), root, "", "", true); code != 0 {
		t.Fatalf("clean --check want 0, got %d", code)
	}
	// tamper index.json -> --check must return drift exit 2
	idx := filepath.Join(root, "index.json")
	orig, _ := os.ReadFile(idx)
	defer os.WriteFile(idx, orig, 0o644)
	_ = os.WriteFile(idx, []byte("{}\n"), 0o644)
	if code := runBuild(filepath.Join(root, "catalog"), root, "", "", true); code != 2 {
		t.Fatalf("drift --check want exit 2, got %d", code)
	}
}

// The RunE closure derives repoRoot as filepath.Dir(filepath.Clean(catalogDir)); none of the
// runBuild unit tests exercise it (they pass repoRoot explicitly). Drive the cobra command so a
// regression in the derivation — including the trailing-slash case — is caught. Against the real
// repo, a correctly-derived repoRoot makes --check clean (exit 0 → RunE returns nil).
func TestBuildCmdDerivesRepoRootFromCatalogArg(t *testing.T) {
	root := repoRoot(t)
	if code := runBuild(filepath.Join(root, "catalog"), root, "", "", false); code != 0 {
		t.Fatalf("pre-build want 0, got %d", code)
	}
	for _, arg := range []string{
		filepath.Join(root, "catalog"),       // plain
		filepath.Join(root, "catalog") + "/", // trailing slash — filepath.Clean must strip it
	} {
		cmd := newBuildCmd()
		cmd.SetArgs([]string{"--check", arg})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("build --check %q: repoRoot mis-derived (got %v)", arg, err)
		}
	}
}

// The --manifest write path (provenance.Compute over out.Files, Marshal, WriteFile, and the
// registry→TargetVersions derivation) is never exercised by the runBuild unit tests, yet
// sign-manifest.yml depends on it (and only runs post-merge). Pin it: a valid, deterministic,
// signable manifest is produced.
func TestBuildWritesManifest(t *testing.T) {
	root := repoRoot(t)
	mp := filepath.Join(t.TempDir(), "build-manifest.json")
	if code := runBuild(filepath.Join(root, "catalog"), root, mp, "", false); code != 0 {
		t.Fatalf("build --manifest want 0, got %d", code)
	}
	b, err := os.ReadFile(mp)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	var m provenance.BuildManifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if m.SchemaVersion != provenance.SchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", m.SchemaVersion, provenance.SchemaVersion)
	}
	if len(m.Files) == 0 {
		t.Fatal("manifest records no files")
	}
	if m.TargetVersions["claude"] < 1 {
		t.Fatalf("claude target version not recorded: %+v", m.TargetVersions)
	}
	// Byte-determinism at the CLI seam (the signed blob must be reproducible).
	mp2 := filepath.Join(t.TempDir(), "build-manifest.json")
	if code := runBuild(filepath.Join(root, "catalog"), root, mp2, "", false); code != 0 {
		t.Fatalf("2nd build want 0, got %d", code)
	}
	b2, _ := os.ReadFile(mp2)
	if string(b) != string(b2) {
		t.Fatal("manifest bytes differ across identical builds (non-deterministic)")
	}
}

func repoRoot(t *testing.T) string {
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "catalog")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("repo root not found")
	return ""
}
