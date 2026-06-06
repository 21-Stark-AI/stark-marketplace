package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCheckExitCodes(t *testing.T) {
	root := repoRoot(t)
	// write a fresh build, then --check must be clean (exit 0)
	if code := runBuild(filepath.Join(root, "catalog"), root, false); code != 0 {
		t.Fatalf("build write want 0, got %d", code)
	}
	if code := runBuild(filepath.Join(root, "catalog"), root, true); code != 0 {
		t.Fatalf("clean --check want 0, got %d", code)
	}
	// tamper index.json -> --check must return drift exit 2
	idx := filepath.Join(root, "index.json")
	orig, _ := os.ReadFile(idx)
	defer os.WriteFile(idx, orig, 0o644)
	_ = os.WriteFile(idx, []byte("{}\n"), 0o644)
	if code := runBuild(filepath.Join(root, "catalog"), root, true); code != 2 {
		t.Fatalf("drift --check want exit 2, got %d", code)
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
