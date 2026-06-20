package gemini

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestGeminiEmulatesSkillIntoGeminiMd(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "PR review.", Version: "0.7.0",
		Body:     "Review carefully.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := find(files, "GEMINI.md")
	if !ok {
		t.Fatalf("expected GEMINI.md; got %v", files)
	}
	if !strings.Contains(body, "EMULATED from stark-review/stark-review") {
		t.Fatalf("missing fidelity header: %q", body)
	}
	if !strings.Contains(body, "<!-- stark:begin stark-review/stark-review@") {
		t.Fatalf("missing begin sentinel: %q", body)
	}
	if !strings.Contains(body, "<!-- stark:end stark-review/stark-review -->") {
		t.Fatalf("missing end sentinel: %q", body)
	}
	if !strings.Contains(body, "Review carefully.") {
		t.Fatalf("body missing: %q", body)
	}
}

func TestGeminiAgentEmulationIsRoleBlock(t *testing.T) {
	a := &model.Artifact{
		Name: "red-team", Type: model.TypeAgent, Bundle: "stark-review",
		Description: "Adversarial reviewer.", Version: "0.7.0",
		Body:     "Attack the design.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, _ := New().Render(bundleWith(a))
	body, _ := find(files, "GEMINI.md")
	if !strings.Contains(body, "Role: red-team") {
		t.Fatalf("agent emulation should render a role block: %q", body)
	}
}
