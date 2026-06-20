package gemini

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func find(files []adapter.OutputFile, suffix string) (string, bool) {
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

func TestGeminiEmitsCommandToml(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand, Bundle: "stark-review",
		Description: "PR review command.", Version: "0.7.0",
		ArgumentHint: "[PR_NUMBER]", Body: "Review the PR for {{args}}.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := find(files, ".gemini/commands/review.toml")
	if !ok {
		t.Fatalf("expected .gemini/commands/review.toml; got %v", files)
	}
	// go-toml/v2 emits a literal (single-quoted) string for the simple description;
	// the prompt carries newlines so it gets a double-quoted basic string.
	if !strings.Contains(body, `description = 'PR review command.'`) {
		t.Fatalf("missing description: %q", body)
	}
	if !strings.Contains(body, "prompt =") || !strings.Contains(body, "{{args}}") {
		t.Fatalf("missing prompt/{{args}}: %q", body)
	}
	if !strings.Contains(body, "[PR_NUMBER]") {
		t.Fatalf("derived arg-hint missing: %q", body)
	}
	if strings.Contains(body, "model =") {
		t.Fatal("gemini command toml must not carry model")
	}
}
