package fieldmap

import (
	"reflect"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestApplyDropsAndWarns(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand,
		Model: "opus", ArgumentHint: "[PR]", DisableModelInvocation: true,
		AllowedTools: []string{"Bash"},
	}
	// nil fm → typed-field fallback path.
	res := Apply(nil, a, model.RuntimeGemini, codexModelMapNoop)
	// Gemini drops model, disable-model-invocation, allowed-tools → 3 warnings.
	if len(res.Dropped) != 3 {
		t.Fatalf("want 3 dropped fields, got %v", res.Dropped)
	}
	if _, ok := res.Carried["model"]; ok {
		t.Fatal("model should not be carried on gemini")
	}
	if res.Derived["argument-hint"] != "[PR]" {
		t.Fatalf("argument-hint should be derived, got %v", res.Derived)
	}
}

func TestApplyMapsCodexModel(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, Model: "opus"}
	mapper := func(v string) (string, bool) {
		if v == "opus" {
			return "gpt-5-codex", true
		}
		return "", false
	}
	res := Apply(nil, a, model.RuntimeCodex, mapper)
	if res.Carried["model"] != "gpt-5-codex" {
		t.Fatalf("codex model should map opus→gpt-5-codex, got %q", res.Carried["model"])
	}
}

func TestApplyMapMissTargetDrops(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, Model: "weird-model"}
	mapper := func(string) (string, bool) { return "", false }
	res := Apply(nil, a, model.RuntimeCodex, mapper)
	if _, ok := res.Carried["model"]; ok {
		t.Fatal("unmappable model must drop")
	}
	if len(res.Dropped) != 1 || res.Dropped[0] != "model" {
		t.Fatalf("want model dropped, got %v", res.Dropped)
	}
}

// Per-runtime frontmatter overrides (res.Frontmatter) must win over typed fields.
func TestApplyHonorsResolvedOverride(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, Model: "opus"}
	fm := map[string]any{"model": "sonnet"} // override-merged value
	mapper := func(v string) (string, bool) {
		if v == "sonnet" {
			return "gpt-5-codex", true
		}
		return "", false
	}
	res := Apply(fm, a, model.RuntimeCodex, mapper)
	if res.Carried["model"] != "gpt-5-codex" {
		t.Fatalf("override model (sonnet) must drive mapping, got %v", res.Carried["model"])
	}
}

// The agent `tools` field is resolved (§6.2 row 4): best-effort on Codex, drop on Gemini.
func TestApplyResolvesToolsField(t *testing.T) {
	a := &model.Artifact{Name: "rt", Type: model.TypeAgent, Tools: []string{"Bash", "Read"}}

	codex := Apply(nil, a, model.RuntimeCodex, nil)
	got, ok := codex.Carried["tools"].([]string)
	if !ok || !reflect.DeepEqual(got, []string{"Bash", "Read"}) {
		t.Fatalf("codex should carry tools as a []string list, got %#v", codex.Carried["tools"])
	}

	gem := Apply(nil, a, model.RuntimeGemini, nil)
	dropped := false
	for _, d := range gem.Dropped {
		if d == "tools" {
			dropped = true
		}
	}
	if !dropped {
		t.Fatalf("gemini should drop tools, got dropped=%v", gem.Dropped)
	}
}

func codexModelMapNoop(v string) (string, bool) { return v, true }
