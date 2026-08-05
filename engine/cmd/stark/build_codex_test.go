package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/build"
)

func TestReadCodexPluginVersion(t *testing.T) {
	missing := t.TempDir()
	got, err := readCodexPluginVersion(missing)
	if err != nil || got != build.DefaultCodexPluginVersion {
		t.Fatalf("missing VERSION = %q, %v", got, err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("2.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = readCodexPluginVersion(root)
	if err != nil || got != "2.3.4" {
		t.Fatalf("VERSION = %q, %v", got, err)
	}

	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(" \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCodexPluginVersion(root); err == nil {
		t.Fatal("empty VERSION accepted")
	}
}
