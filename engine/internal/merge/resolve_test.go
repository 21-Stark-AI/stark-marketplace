package merge

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestResolveMergesAndStripsFences(t *testing.T) {
	a := &model.Artifact{
		Name: "x", Type: model.TypeCommand, Description: "d", Version: "0.1.0",
		Model:    "opus",
		Runtimes: model.AllRuntimes(),
		Raw:      map[string]any{"model": "opus", "tags": []any{"a", "b"}},
		Body:     "base\n<!-- runtime: claude -->\nC\n<!-- /runtime -->\n",
		Overrides: map[model.Runtime]model.Override{
			model.RuntimeGemini: {Fields: map[string]any{"model": "gemini-2.5-pro"}},
		},
	}
	res, f, err := Resolve(a, model.RuntimeGemini)
	if err != nil {
		t.Fatal(err)
	}
	if res.Frontmatter["model"] != "gemini-2.5-pro" {
		t.Fatalf("override not applied: %v", res.Frontmatter["model"])
	}
	if res.Body != "base\n" { // claude fence stripped for gemini target
		t.Fatalf("body = %q", res.Body)
	}
	if f.Diverged {
		t.Fatal("did not expect divergence")
	}
}

func TestResolveDivergedBodyRequiresReason(t *testing.T) {
	withReason := &model.Artifact{
		Name: "x", Type: model.TypeCommand, Version: "0.1.0", Runtimes: model.AllRuntimes(),
		Raw:  map[string]any{},
		Body: "base\n",
		Overrides: map[model.Runtime]model.Override{
			model.RuntimeCodex: {Body: "# diverged: codex needs different steps\nCodex body\n"},
		},
	}
	res, f, err := Resolve(withReason, model.RuntimeCodex)
	if err != nil {
		t.Fatalf("annotated divergence should be allowed: %v", err)
	}
	if !f.Diverged || f.DivergedReason != "codex needs different steps" {
		t.Fatalf("findings = %+v", f)
	}
	if res.Body != "Codex body\n" {
		t.Fatalf("body = %q", res.Body)
	}

	noReason := &model.Artifact{
		Name: "x", Type: model.TypeCommand, Version: "0.1.0", Runtimes: model.AllRuntimes(),
		Raw: map[string]any{}, Body: "base\n",
		Overrides: map[model.Runtime]model.Override{
			model.RuntimeCodex: {Body: "No annotation here\n"},
		},
	}
	if _, _, err := Resolve(noReason, model.RuntimeCodex); err == nil {
		t.Fatal("unannotated full-body replacement must be a lint error")
	}
}
