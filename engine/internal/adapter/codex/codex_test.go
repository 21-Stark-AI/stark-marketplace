package codex

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func findFile(files []adapter.OutputFile, suffix string) (string, bool) {
	for _, f := range files {
		if strings.HasSuffix(f.Path, suffix) {
			return string(f.Content), true
		}
	}
	return "", false
}

// bundleWith wraps one artifact in a single-artifact bundle for target tests.
func bundleWith(a *model.Artifact) *model.Bundle {
	return &model.Bundle{Name: a.Bundle, Artifacts: []*model.Artifact{a}}
}

func TestCodexEmitsNativeSkill(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "Single-agent PR review.", Version: "0.7.0",
		Body:     "Do the review.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, ".agents/skills/stark-review/SKILL.md")
	if !ok {
		t.Fatalf("expected native Codex skill path; got %v", files)
	}
	if !contains(body, "name: stark-review") || !contains(body, "description: Single-agent PR review.") {
		t.Fatalf("missing required frontmatter: %q", body)
	}
	if contains(body, "EMULATED from") {
		t.Fatal("native skill must NOT carry an emulation header")
	}
	if !contains(body, "Do the review.") {
		t.Fatalf("body missing: %q", body)
	}
}

func TestCodexMapsCommandToSkillWithUsage(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand, Bundle: "stark-review",
		Description: "PR review command.", Version: "0.7.0",
		ArgumentHint: "[PR_NUMBER]", Body: "Review the PR.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, ".agents/skills/review/SKILL.md")
	if !ok {
		t.Fatalf("command must map to a Codex skill; got %v", files)
	}
	if !contains(body, "Usage:") || !contains(body, "[PR_NUMBER]") {
		t.Fatalf("derived usage missing: %q", body)
	}
}

func TestCodexAgentEmulationHasHeader(t *testing.T) {
	a := &model.Artifact{
		Name: "red-team", Type: model.TypeAgent, Bundle: "stark-review",
		Description: "Adversarial reviewer.", Version: "0.7.0",
		Body:     "Attack the design.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, ".agents/skills/red-team/SKILL.md")
	if !ok {
		t.Fatalf("agent must emulate as a Codex skill; got %v", files)
	}
	if !contains(body, "EMULATED from stark-review/red-team") {
		t.Fatalf("emulated agent must carry fidelity header: %q", body)
	}
}
