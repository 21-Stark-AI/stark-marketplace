package validate

import (
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/model"
)

func TestOpenAICompatibilityWarnsOnClaudeOnlySkills(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, Runtimes: []model.Runtime{model.RuntimeClaude}}
	r := &Result{}
	checkOpenAICompatibility(r, "demo/skill/s", a)
	if r.HasErrors() {
		t.Fatalf("claude-only is a deliberate declaration, not an error: %+v", r.Errors)
	}
	if len(r.Warnings) != 1 {
		t.Fatalf("warnings = %+v, want exactly the claude-only notice", r.Warnings)
	}
}

func TestOpenAICompatibilityWarnsOnClaudeOnlyCommands(t *testing.T) {
	a := &model.Artifact{Name: "cmd", Type: model.TypeCommand, Runtimes: []model.Runtime{model.RuntimeClaude}}
	r := &Result{}
	checkOpenAICompatibility(r, "demo/command/cmd", a)
	if r.HasErrors() {
		t.Fatalf("claude-only is a deliberate declaration, not an error: %+v", r.Errors)
	}
	if len(r.Warnings) != 1 {
		t.Fatalf("warnings = %+v, want exactly the claude-only notice", r.Warnings)
	}
}

func TestOpenAICompatibilityAllowsCodexParity(t *testing.T) {
	for _, typ := range []model.ArtifactType{model.TypeSkill, model.TypeCommand} {
		a := &model.Artifact{
			Name:     "x",
			Type:     typ,
			Runtimes: []model.Runtime{model.RuntimeClaude, model.RuntimeCodex},
		}
		r := &Result{}
		checkOpenAICompatibility(r, "demo/"+string(typ)+"/x", a)
		if r.HasErrors() {
			t.Fatalf("%s with codex parity should pass: %+v", typ, r.Errors)
		}
	}
}

func TestOpenAICompatibilityDoesNotApplyToOtherTypes(t *testing.T) {
	a := &model.Artifact{Name: "agent", Type: model.TypeAgent, Runtimes: []model.Runtime{model.RuntimeClaude}}
	r := &Result{}
	checkOpenAICompatibility(r, "demo/agent/agent", a)
	if r.HasErrors() {
		t.Fatalf("non skill/command artifacts are governed by the capability matrix: %+v", r.Errors)
	}
}
