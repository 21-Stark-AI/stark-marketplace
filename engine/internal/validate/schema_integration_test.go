package validate

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestCatalogRunsSchema(t *testing.T) {
	a := &model.Artifact{
		Name: "x", Type: model.TypeCommand, Runtimes: model.AllRuntimes(),
		Raw: map[string]any{"name": "x", "type": "command"}, // missing description+version
	}
	b := &model.Bundle{Name: "demo", Version: "0.1.0", Description: "d",
		Owner: model.Owner{Name: "E"}, Runtimes: model.AllRuntimes(), Artifacts: []*model.Artifact{a}}
	r := Catalog(&model.Catalog{Bundles: []*model.Bundle{b}})
	if !r.HasErrors() {
		t.Fatal("expected schema error for missing required fields")
	}
}
