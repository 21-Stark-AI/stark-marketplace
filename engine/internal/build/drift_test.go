package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
)

func TestCheckReportsDriftOnTamper(t *testing.T) {
	root := t.TempDir()
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Build(cat, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(root, out); err != nil {
		t.Fatal(err)
	}
	// clean check: no drift
	if drift, err := Check(root, out); err != nil || len(drift) != 0 {
		t.Fatalf("expected no drift, got %v err %v", drift, err)
	}
	// tamper one file
	tampered := filepath.Join(root, "index.json")
	if err := os.WriteFile(tampered, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := Check(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) == 0 {
		t.Fatal("expected drift after tamper")
	}
}

func TestCheckReportsMissingAndExtra(t *testing.T) {
	root := t.TempDir()
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Build(cat, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(root, out); err != nil {
		t.Fatal(err)
	}

	// (1) missing branch: delete an expected file.
	if err := os.Remove(filepath.Join(root, "index.json")); err != nil {
		t.Fatal(err)
	}
	drift, err := Check(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSuffix(drift, "(missing)") {
		t.Fatalf("expected a (missing) drift entry, got %v", drift)
	}

	// restore, then (2) unexpected/extra branch: write a stray file under a generated root.
	if err := os.WriteFile(filepath.Join(root, "index.json"), out.Files["index.json"], 0o644); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(root, "dist", "claude", "stark-gh", "extra.md")
	if err := os.WriteFile(stray, []byte("stray\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift2, err := Check(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSuffix(drift2, "(unexpected)") {
		t.Fatalf("expected an (unexpected) drift entry, got %v", drift2)
	}
}

func containsSuffix(items []string, suffix string) bool {
	for _, s := range items {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}
