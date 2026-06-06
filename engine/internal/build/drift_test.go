package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
)

func TestCheckReportsDriftOnTamper(t *testing.T) {
	root := t.TempDir()
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Build(cat)
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
