package load

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestLoadCatalog(t *testing.T) {
	cat, err := Load("testdata/catalog")
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Bundles) != 1 {
		t.Fatalf("want 1 bundle, got %d", len(cat.Bundles))
	}
	b := cat.Bundles[0]
	if b.Name != "demo" || len(b.Artifacts) != 1 {
		t.Fatalf("bundle = %+v", b)
	}
	a := b.Artifacts[0]
	if a.Name != "hello" || a.Type != model.TypeCommand {
		t.Fatalf("artifact = %+v", a)
	}
	// inheritance: category/tags/runtimes come from the bundle
	if a.Category != "examples" || len(a.Runtimes) != 3 {
		t.Fatalf("inheritance failed: %+v", a)
	}
	if a.Body != "Hello, world.\n" {
		t.Fatalf("body = %q", a.Body)
	}
}
