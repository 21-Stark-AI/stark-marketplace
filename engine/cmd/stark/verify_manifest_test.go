package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/provenance"
)

func TestVerifyManifestDigestsOnly(t *testing.T) {
	dir := t.TempDir()
	// committed bytes
	idx := filepath.Join(dir, "index.json")
	if err := os.WriteFile(idx, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// manifest over those bytes
	m := provenance.Compute(map[string]int{"claude": 1},
		map[string][]byte{"index.json": []byte("hello")})
	mb, _ := m.Marshal()
	mp := filepath.Join(dir, "build-manifest.json")
	if err := os.WriteFile(mp, mb, 0o644); err != nil {
		t.Fatal(err)
	}

	// --skip-signature so the test does not require cosign on PATH; digest layer runs.
	if code := runVerifyManifest(mp, dir, true); code != 0 {
		t.Fatalf("want exit 0 for matching digests, got %d", code)
	}

	// tamper → integrity exit 3 (spec §9.8)
	if err := os.WriteFile(idx, []byte("TAMPERED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runVerifyManifest(mp, dir, true); code != 3 {
		t.Fatalf("want exit 3 on digest mismatch, got %d", code)
	}
}

// Load/parse failures map to exit 1 (spec §9.8), distinct from integrity exit 3.
func TestVerifyManifestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if code := runVerifyManifest(filepath.Join(dir, "nope.json"), dir, true); code != 1 {
		t.Fatalf("missing manifest want exit 1, got %d", code)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runVerifyManifest(bad, dir, true); code != 1 {
		t.Fatalf("malformed manifest want exit 1, got %d", code)
	}
}

// With signature verification enabled but no .sig/.pem present (or cosign absent), the cosign
// step fails and must map to integrity exit 3 — never 0 and never a crash.
func TestVerifyManifestSignatureFailureExits3(t *testing.T) {
	dir := t.TempDir()
	m := provenance.Compute(map[string]int{"claude": 1}, map[string][]byte{"index.json": []byte("hi")})
	mb, _ := m.Marshal()
	mp := filepath.Join(dir, "build-manifest.json")
	if err := os.WriteFile(mp, mb, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runVerifyManifest(mp, dir, false); code != 3 {
		t.Fatalf("signature step failure want exit 3, got %d", code)
	}
}

// Round-trip: a manifest produced by `stark build --manifest` verifies clean against the same
// committed tree (digest layer). Pins the build↔verify format contract the signing flow relies on.
func TestVerifyManifestRoundTripFromBuild(t *testing.T) {
	root := tempRepoRoot(t)
	mp := filepath.Join(t.TempDir(), "build-manifest.json")
	if code := runBuild(filepath.Join(root, "catalog"), root, mp, "", false); code != 0 {
		t.Fatalf("build --manifest want 0, got %d", code)
	}
	if code := runVerifyManifest(mp, root, true); code != 0 {
		t.Fatalf("round-trip verify want 0, got %d", code)
	}
}
