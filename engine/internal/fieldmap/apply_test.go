package fieldmap

import (
	"reflect"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/model"
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

func TestApplyDropsCodexModel(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, Model: "opus"}
	res := Apply(nil, a, model.RuntimeCodex, nil)
	if _, ok := res.Carried["model"]; ok {
		t.Fatal("Codex skill model must drop")
	}
	if len(res.Dropped) != 1 || res.Dropped[0] != "model" {
		t.Fatalf("want model dropped, got %v", res.Dropped)
	}
}

func TestApplyDerivesCodexInvocationPolicy(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, DisableModelInvocation: true}
	res := Apply(nil, a, model.RuntimeCodex, nil)
	if got := res.Derived["disable-model-invocation"]; got != "true" {
		t.Fatalf("disable-model-invocation should derive to metadata, got %q", got)
	}
	if len(res.Dropped) != 0 {
		t.Fatalf("mapped invocation policy must not be reported dropped: %v", res.Dropped)
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
