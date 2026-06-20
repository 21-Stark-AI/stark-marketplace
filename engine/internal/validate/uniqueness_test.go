package validate

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
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

func TestCodexAgentFoldsIntoSkillNamespace(t *testing.T) {
	// CC-7: on Codex, skill and agent share one namespace (both → .agents/skills/).
	if outputNamespace(model.TypeSkill, model.RuntimeCodex) != outputNamespace(model.TypeAgent, model.RuntimeCodex) {
		t.Fatal("codex agent must share the skill-like namespace (CC-7)")
	}
	// Guard: the fold is Codex-only. On Gemini, skill/agent emulate into distinct
	// GEMINI.md sentinel blocks (no filename collision) → distinct namespaces.
	if outputNamespace(model.TypeSkill, model.RuntimeGemini) == outputNamespace(model.TypeAgent, model.RuntimeGemini) {
		t.Fatal("gemini skill/agent emulate to distinct sentinel blocks; must NOT share a namespace")
	}
}

func TestCodexSkillAgentNameCollision(t *testing.T) {
	// A Codex skill "x" and agent "x" overwrite each other → must error (CC-7).
	b := &model.Bundle{Name: "demo", Runtimes: []model.Runtime{model.RuntimeCodex}, Artifacts: []*model.Artifact{
		{Name: "x", Type: model.TypeSkill, Runtimes: []model.Runtime{model.RuntimeCodex}},
		{Name: "x", Type: model.TypeAgent, Runtimes: []model.Runtime{model.RuntimeCodex}},
	}}
	r := &Result{}
	checkOutputUniqueness(r, b)
	if !r.HasErrors() {
		t.Fatal("codex skill+agent name collision must error (CC-7)")
	}
}
