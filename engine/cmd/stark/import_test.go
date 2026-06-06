package main

import (
	"os"
	"path/filepath"
	"testing"
)

func fixtureFrom(t *testing.T) string {
	root := findRepoRoot(t) // defined in validate_test.go (plan 01 Task 15)
	return filepath.Join(root, "engine", "internal", "importer", "testdata", "stark-skills")
}

func TestImportDryRunWritesNothing(t *testing.T) {
	dst := t.TempDir()
	code := runImport(importOpts{from: fixtureFrom(t), bundle: "stark-gh", dest: dst, dryRun: true})
	if code != 0 {
		t.Fatalf("dry-run exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(dst, "stark-gh")); !os.IsNotExist(err) {
		t.Fatal("dry-run must not write the bundle dir")
	}
}

func TestImportWritesBundle(t *testing.T) {
	dst := t.TempDir()
	code := runImport(importOpts{from: fixtureFrom(t), bundle: "stark-gh", dest: dst})
	if code != 0 {
		t.Fatalf("import exit = %d", code)
	}
	for _, p := range []string{"bundle.yaml", "commands/pr-open.md", "mcp/gh.yaml", "IMPORT-NOTES.md"} {
		if _, err := os.Stat(filepath.Join(dst, "stark-gh", p)); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
}
