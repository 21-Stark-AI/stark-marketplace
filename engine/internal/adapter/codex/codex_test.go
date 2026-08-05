package codex

import (
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/adapter"
	"github.com/21StarkCom/bifrost/engine/internal/model"
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

func TestOpenAIShortDescriptionAlwaysFitsUIGuidance(t *testing.T) {
	for _, description := range []string{
		"x",
		"PR review.",
		"A normal description that already fits the expected interface size.",
		strings.Repeat("long description ", 20),
	} {
		got := shortDescription(description)
		n := len([]rune(got))
		if n < 25 || n > 64 {
			t.Fatalf("shortDescription(%q) produced %d chars: %q", description, n, got)
		}
	}
}

func TestCodexDescriptionsTranslateKnownInvocationSyntax(t *testing.T) {
	build := &model.Artifact{
		Name: "stark-build", Type: model.TypeSkill, Bundle: "stark-implement",
		Description: "Run /stark-build, then /stark-copilot; keep /claude-only, docs/stark-build, ../stark-build, ./stark-build, and /stark-build/SKILL.md unchanged.",
		Version:     "0.1.0", DisableModelInvocation: true, Body: "Build.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	b := &model.Bundle{Name: "stark-implement", Artifacts: []*model.Artifact{
		build,
		{Name: "stark-copilot", Type: model.TypeSkill, Bundle: "stark-implement", Description: "Pair.", Version: "0.1.0", Body: "Pair.\n", Runtimes: []model.Runtime{model.RuntimeCodex}},
		{Name: "claude-only", Type: model.TypeCommand, Bundle: "stark-implement", Description: "Claude.", Version: "0.1.0", Body: "Claude.\n", Runtimes: []model.Runtime{model.RuntimeClaude}},
	}}

	files, _, err := NewPlugin().Render(b)
	if err != nil {
		t.Fatal(err)
	}
	skill, ok := findFile(files, "skills/stark-build/SKILL.md")
	if !ok {
		t.Fatalf("rendered skill missing: %v", files)
	}
	metadata, ok := findFile(files, "skills/stark-build/agents/openai.yaml")
	if !ok {
		t.Fatalf("rendered interface metadata missing: %v", files)
	}
	for path, body := range map[string]string{"SKILL.md": skill, "agents/openai.yaml": metadata} {
		for _, want := range []string{"$stark-build", "$stark-copilot"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s missing %q:\n%s", path, want, body)
			}
		}
		for _, unwanted := range []string{"Run /stark-build,", "then /stark-copilot;"} {
			if strings.Contains(body, unwanted) {
				t.Errorf("%s retained Claude invocation %q:\n%s", path, unwanted, body)
			}
		}
	}
	for _, want := range []string{"/claude-only", "docs/stark-build", "../stark-build", "./stark-build", "/stark-build/SKILL.md"} {
		if !strings.Contains(skill, want) {
			t.Errorf("SKILL.md changed non-Codex reference %q:\n%s", want, skill)
		}
	}
}

func TestCodexDescriptionsTranslateQualifiedCommandsTypeAware(t *testing.T) {
	in := "Run /stark-gh:cleanup, /stark-gh:pr-open, or /stark-gh:pr-merge; keep /stark-gh:stark-review."
	want := "Run $cleanup, $pr-open, or $pr-merge; keep /stark-gh:stark-review."
	cleanup := &model.Artifact{
		Name: "cleanup", Type: model.TypeCommand, Bundle: "stark-gh", Description: in,
		Version: "0.1.0", Body: "Clean.\n", Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	b := &model.Bundle{Name: "stark-gh", Artifacts: []*model.Artifact{
		cleanup,
		{Name: "pr-open", Type: model.TypeCommand, Runtimes: []model.Runtime{model.RuntimeCodex}},
		{Name: "pr-merge", Type: model.TypeCommand, Runtimes: []model.Runtime{model.RuntimeCodex}},
		{Name: "stark-review", Type: model.TypeSkill, Runtimes: []model.Runtime{model.RuntimeCodex}},
	}}
	if got := TranslateInvocationReferences(in, b); got != want {
		t.Fatalf("qualified invocation translation:\n got: %q\nwant: %q", got, want)
	}

	files, _, err := NewPlugin().Render(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"skills/cleanup/SKILL.md", "skills/cleanup/agents/openai.yaml"} {
		body, ok := findFile(files, path)
		if !ok || !strings.Contains(body, "$cleanup") || !strings.Contains(body, "$pr-open") || !strings.Contains(body, "$pr-merge") {
			t.Errorf("%s did not project qualified commands into its description:\n%s", path, body)
		}
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
	metadata, ok := findFile(files, ".agents/skills/review/agents/openai.yaml")
	if !ok || !contains(metadata, "allow_implicit_invocation: false") {
		t.Fatalf("command emulation must stay explicit-only, got %q", metadata)
	}
}

func TestCodexCommandExplicitFalseRemainsExplicitOnly(t *testing.T) {
	a := &model.Artifact{
		Name: "cleanup", Type: model.TypeCommand, Bundle: "stark-gh",
		Description: "Clean up repository state.", Version: "0.7.0",
		DisableModelInvocation: false,
		Raw:                    map[string]any{"disable-model-invocation": false},
		Body:                   "Clean up.\n",
		Runtimes:               []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	metadata, ok := findFile(files, ".agents/skills/cleanup/agents/openai.yaml")
	if !ok {
		t.Fatalf("command metadata missing: %v", files)
	}
	if !contains(metadata, "allow_implicit_invocation: false") {
		t.Fatalf("explicit false frontmatter weakened command policy:\n%s", metadata)
	}
}

func TestCodexDropsUnsupportedSkillModel(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "PR review.", Version: "0.7.0",
		Model:    "opus[1m]",
		Body:     "Review.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	files, findings, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, ".agents/skills/review/SKILL.md")
	if !ok {
		t.Fatalf("expected native Codex skill path; got %v", files)
	}
	if contains(body, "model:") {
		t.Fatalf("Codex SKILL.md must not carry unsupported model metadata: %q", body)
	}
	dropped := false
	for _, f := range findings {
		if contains(f.Msg, `field "model" dropped`) {
			dropped = true
		}
	}
	if !dropped {
		t.Fatalf("dropping authored model should remain visible: %+v", findings)
	}
}

func TestCodexMapsExplicitOnlyPolicyToOpenAIMetadata(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-release", Type: model.TypeSkill, Bundle: "stark-ops",
		Description: "Cut a release from the current repository.", Version: "0.7.0",
		DisableModelInvocation: true,
		Body:                   "Cut the release.\n",
		Runtimes:               []model.Runtime{model.RuntimeCodex},
	}
	files, findings, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	metadata, ok := findFile(files, ".agents/skills/stark-release/agents/openai.yaml")
	if !ok {
		t.Fatalf("missing Codex skill metadata: %v", files)
	}
	for _, want := range []string{
		`display_name: "Stark Release"`,
		`short_description: "Cut a release from the current repository."`,
		"allow_implicit_invocation: false",
	} {
		if !contains(metadata, want) {
			t.Errorf("metadata missing %q:\n%s", want, metadata)
		}
	}
	for _, f := range findings {
		if contains(f.Msg, "disable-model-invocation") {
			t.Fatalf("native policy must not be reported dropped: %+v", findings)
		}
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
