package fieldmap

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestApplyDropsAndWarns(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand,
		Model: "opus", ArgumentHint: "[PR]", DisableModelInvocation: true,
		AllowedTools: []string{"Bash"},
	}
	res := Apply(a, model.RuntimeGemini, codexModelMapNoop)
	// Gemini drops model, disable-model-invocation, allowed-tools → 3 warnings.
	if len(res.Dropped) != 3 {
		t.Fatalf("want 3 dropped fields, got %v", res.Dropped)
	}
	if _, ok := res.Carried["model"]; ok {
		t.Fatal("model should not be carried on gemini")
	}
	// argument-hint is derived, not carried, not dropped.
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
	res := Apply(a, model.RuntimeCodex, mapper)
	if res.Carried["model"] != "gpt-5-codex" {
		t.Fatalf("codex model should map opus→gpt-5-codex, got %q", res.Carried["model"])
	}
}

func TestApplyMapMissTargetDrops(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, Model: "weird-model"}
	mapper := func(string) (string, bool) { return "", false }
	res := Apply(a, model.RuntimeCodex, mapper)
	if _, ok := res.Carried["model"]; ok {
		t.Fatal("unmappable model must drop")
	}
	if len(res.Dropped) != 1 || res.Dropped[0] != "model" {
		t.Fatalf("want model dropped, got %v", res.Dropped)
	}
}

func codexModelMapNoop(v string) (string, bool) { return v, true }
