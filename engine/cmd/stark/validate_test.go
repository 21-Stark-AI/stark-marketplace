package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCommandExitZeroOnSeed(t *testing.T) {
	root := findRepoRoot(t)
	code := runValidate(filepath.Join(root, "catalog"))
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
}

func findRepoRoot(t *testing.T) string {
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
