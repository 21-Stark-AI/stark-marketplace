package claude

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestRenderSkillCommandAgentPaths(t *testing.T) {
	b := &model.Bundle{Name: "demo", Runtimes: []model.Runtime{model.RuntimeClaude}, Artifacts: []*model.Artifact{
		{Name: "rev", Type: model.TypeSkill, Description: "review skill", Version: "0.1.0",
			Runtimes: []model.Runtime{model.RuntimeClaude}, Raw: map[string]any{}, Body: "Skill body\n"},
		{Name: "review", Type: model.TypeCommand, Description: "review cmd", Version: "0.1.0",
			ArgumentHint: "[PR]", Runtimes: []model.Runtime{model.RuntimeClaude}, Raw: map[string]any{}, Body: "Cmd body\n"},
		{Name: "rt", Type: model.TypeAgent, Description: "red team", Version: "0.1.0",
			Tools: []string{"Bash"}, Model: "opus", Runtimes: []model.Runtime{model.RuntimeClaude},
			Raw: map[string]any{}, Body: "Agent body\n"},
	}}
	files, _, err := New().Render(b)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Content)
	}
	if _, ok := byPath["skills/rev/SKILL.md"]; !ok {
		t.Fatalf("missing skill path; got %v", keys(byPath))
	}
	cmd, ok := byPath["commands/review.md"]
	if !ok || !strings.Contains(cmd, "argument-hint: '[PR]'") || !strings.Contains(cmd, "Cmd body") {
		t.Fatalf("command wrong: %q", cmd)
	}
	ag, ok := byPath["agents/rt.md"]
	if !ok || !strings.Contains(ag, "tools:") || !strings.Contains(ag, "Agent body") {
		t.Fatalf("agent wrong: %q", ag)
	}
}

func keys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// TestRenderExplicitFalseBooleanSurvives proves an authored
// `disable-model-invocation: false` in the source frontmatter reaches output
// rather than being silently dropped (F-Cov#9 — booleans present in the resolved
// frontmatter are emitted explicitly).
func TestRenderExplicitFalseBooleanSurvives(t *testing.T) {
	b := &model.Bundle{Name: "demo", Runtimes: []model.Runtime{model.RuntimeClaude}, Artifacts: []*model.Artifact{
		{Name: "rev", Type: model.TypeSkill, Description: "s", Version: "0.1.0",
			Runtimes: []model.Runtime{model.RuntimeClaude},
			Raw:      map[string]any{"disable-model-invocation": false}, Body: "Skill body\n"},
	}}
	files, _, err := New().Render(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "skills/rev/SKILL.md" {
			if !strings.Contains(string(f.Content), "disable-model-invocation: false") {
				t.Fatalf("explicit false boolean dropped from output:\n%s", f.Content)
			}
			return
		}
	}
	t.Fatal("skill file not emitted")
}
