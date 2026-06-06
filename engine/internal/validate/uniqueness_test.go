package validate

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestOutputUniquenessCollision(t *testing.T) {
	// On Codex, both a skill and a command named "x" could map to the same
	// prompts/x namespace — collision must be caught.
	b := &model.Bundle{Name: "demo", Runtimes: model.AllRuntimes(), Artifacts: []*model.Artifact{
		{Name: "x", Type: model.TypeCommand, Runtimes: model.AllRuntimes()},
		{Name: "x", Type: model.TypePrompt, Runtimes: model.AllRuntimes()},
	}}
	r := &Result{}
	checkOutputUniqueness(r, b)
	if !r.HasErrors() {
		t.Fatal("expected output-namespace collision error")
	}
}

func TestOutputUniquenessNoFalsePositive(t *testing.T) {
	// Claude keeps types in separate namespaces, so a skill and a command both named
	// "x" targeting Claude-only must NOT collide. Locks the per-runtime separation
	// that is the whole point of outputNamespace (guards against a too-broad regression).
	b := &model.Bundle{Name: "demo", Runtimes: []model.Runtime{model.RuntimeClaude}, Artifacts: []*model.Artifact{
		{Name: "x", Type: model.TypeSkill, Runtimes: []model.Runtime{model.RuntimeClaude}},
		{Name: "x", Type: model.TypeCommand, Runtimes: []model.Runtime{model.RuntimeClaude}},
	}}
	r := &Result{}
	checkOutputUniqueness(r, b)
	if r.HasErrors() {
		t.Fatalf("claude separates types; should not collide: %+v", r.Errors)
	}

	// distinct names never collide, regardless of runtime folding.
	b2 := &model.Bundle{Name: "demo", Runtimes: model.AllRuntimes(), Artifacts: []*model.Artifact{
		{Name: "a", Type: model.TypeSkill, Runtimes: model.AllRuntimes()},
		{Name: "b", Type: model.TypeCommand, Runtimes: model.AllRuntimes()},
	}}
	r2 := &Result{}
	checkOutputUniqueness(r2, b2)
	if r2.HasErrors() {
		t.Fatalf("distinct names should not collide: %+v", r2.Errors)
	}
}
