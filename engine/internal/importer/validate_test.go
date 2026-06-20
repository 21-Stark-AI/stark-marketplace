package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/validate"
)

func TestImportedBundleValidatesClean(t *testing.T) {
	res, err := Import(Options{From: "testdata/stark-skills", Bundle: "stark-gh"})
	if err != nil {
		t.Fatal(err)
	}
	catRoot := t.TempDir()
	if err := WriteBundle(res, catRoot); err != nil {
		t.Fatal(err)
	}

	// load the written catalog through the real loader, validate through the real runner
	cat, err := load.Load(catRoot)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	r := validate.Catalog(cat)
	if r.HasErrors() {
		// dump the bundle.yaml for debugging
		dump, _ := os.ReadFile(filepath.Join(catRoot, "stark-gh", "bundle.yaml"))
		t.Fatalf("imported bundle has validation errors: %+v\nbundle.yaml:\n%s", r.Errors, dump)
	}
}

func TestImportedSkillBundleValidatesClean(t *testing.T) {
	res, err := Import(Options{From: "testdata/stark-skills", Bundle: "demo-skills"})
	if err != nil {
		t.Fatal(err)
	}
	catRoot := t.TempDir()
	if err := WriteBundle(res, catRoot); err != nil {
		t.Fatal(err)
	}
	cat, err := load.Load(catRoot)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r := validate.Catalog(cat); r.HasErrors() {
		t.Fatalf("imported skill bundle has errors: %+v", r.Errors)
	}
}
