package validate

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestResultErrorsAndWarnings(t *testing.T) {
	r := &Result{}
	r.Errorf("a", "boom %d", 1)
	r.Warnf("a", "careful")
	if !r.HasErrors() {
		t.Fatal("want errors")
	}
	if len(r.Warnings) != 1 {
		t.Fatalf("want 1 warning, got %d", len(r.Warnings))
	}
}

func TestValidateCatalogRunsRules(t *testing.T) {
	cat := &model.Catalog{Bundles: []*model.Bundle{{
		Name: "BAD_NAME", Version: "0.1.0", Description: "x",
		Owner: model.Owner{Name: "E"},
	}}}
	r := Catalog(cat)
	if !r.HasErrors() {
		t.Fatal("expected slug error for BAD_NAME")
	}
}

func TestRuntimesNarrowing(t *testing.T) {
	b := &model.Bundle{Name: "demo", Runtimes: []model.Runtime{model.RuntimeClaude}}
	a := &model.Artifact{Name: "x", Type: model.TypeCommand,
		Runtimes: []model.Runtime{model.RuntimeClaude, model.RuntimeGemini}} // widens!
	r := &Result{}
	checkRuntimesNarrowing(r, "demo/command/x", a, b)
	if !r.HasErrors() {
		t.Fatal("expected widening error")
	}
}
