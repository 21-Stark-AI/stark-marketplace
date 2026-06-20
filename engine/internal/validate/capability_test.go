package validate

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestCapabilityWarnsOnEmulated(t *testing.T) {
	// agent on codex is emulated → warning, not error.
	a := &model.Artifact{Name: "rt", Type: model.TypeAgent,
		Runtimes: []model.Runtime{model.RuntimeCodex}}
	r := &Result{}
	checkCapability(r, "demo/agent/rt", a)
	if r.HasErrors() {
		t.Fatalf("emulated should not error: %+v", r.Errors)
	}
	if len(r.Warnings) != 1 {
		t.Fatalf("want 1 emulated warning, got %d", len(r.Warnings))
	}
}

func TestCapabilityErrorsOnUnsupported(t *testing.T) {
	// an unknown type is unsupported on every runtime.
	a := &model.Artifact{Name: "x", Type: model.ArtifactType("widget"),
		Runtimes: []model.Runtime{model.RuntimeGemini}}
	r := &Result{}
	checkCapability(r, "demo/widget/x", a)
	if !r.HasErrors() {
		t.Fatal("unsupported (type,runtime) must error")
	}
}

func TestCapabilityNativeIsSilent(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill,
		Runtimes: []model.Runtime{model.RuntimeClaude}}
	r := &Result{}
	checkCapability(r, "demo/skill/s", a)
	if r.HasErrors() || len(r.Warnings) != 0 {
		t.Fatalf("native should be silent: %+v / %+v", r.Errors, r.Warnings)
	}
}
