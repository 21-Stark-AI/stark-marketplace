package validate

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
)

func TestSeedCatalogIsValid(t *testing.T) {
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	r := Catalog(cat)
	if r.HasErrors() {
		t.Fatalf("seed catalog has errors: %+v", r.Errors)
	}
}
