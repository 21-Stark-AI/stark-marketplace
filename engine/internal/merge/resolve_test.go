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

func TestResolveReportsArrayFootgunThroughResolve(t *testing.T) {
	// An override `tags` array that drops a base prefix element must surface an
	// ArrayDrops finding via the public Resolve path (not just the private helper).
	a := &model.Artifact{
		Name: "x", Type: model.TypeCommand, Version: "0.1.0", Runtimes: model.AllRuntimes(),
		Raw:  map[string]any{"tags": []any{"a", "b", "c"}},
		Body: "body\n",
		Overrides: map[model.Runtime]model.Override{
			model.RuntimeGemini: {Fields: map[string]any{"tags": []any{"a", "c"}}},
		},
	}
	_, f, err := Resolve(a, model.RuntimeGemini)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.ArrayDrops) != 1 || f.ArrayDrops[0] != "tags" {
		t.Fatalf("want ArrayDrops=[tags], got %v", f.ArrayDrops)
	}
}

func TestResolveReportsNestedMCPArgsFootgun(t *testing.T) {
	// spec §4.3 names mcp.args explicitly: dropping a prefix element of the nested
	// args array must warn, even though it lives under the mcp sub-map.
	a := &model.Artifact{
		Name: "m", Type: model.TypeMCP, Version: "0.1.0", Runtimes: model.AllRuntimes(),
		Raw: map[string]any{"mcp": map[string]any{"args": []any{"server.js", "--port", "3000"}}},
		Overrides: map[model.Runtime]model.Override{
			model.RuntimeCodex: {Fields: map[string]any{"mcp": map[string]any{"args": []any{"server.js"}}}},
		},
	}
	_, f, err := Resolve(a, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range f.ArrayDrops {
		if d == "mcp.args" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want ArrayDrops to include mcp.args, got %v", f.ArrayDrops)
	}
}

func TestResolveRequiresArrayDoesNotPanic(t *testing.T) {
	// `requires` entries are map[string]any; the foot-gun equality check must not
	// panic comparing them (regression for the == on maps crash).
	a := &model.Artifact{
		Name: "x", Type: model.TypeCommand, Version: "0.1.0", Runtimes: model.AllRuntimes(),
		Raw: map[string]any{"requires": []any{
			map[string]any{"type": "skill", "ref": "foo"},
			map[string]any{"type": "skill", "ref": "bar"},
		}},
		Body: "body\n",
		Overrides: map[model.Runtime]model.Override{
			model.RuntimeGemini: {Fields: map[string]any{"requires": []any{
				map[string]any{"type": "skill", "ref": "foo"},
			}}},
		},
	}
	if _, _, err := Resolve(a, model.RuntimeGemini); err != nil {
		t.Fatal(err)
	}
}
