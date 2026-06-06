package index

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestDetailCoversAllTargetedRuntimes(t *testing.T) {
	// An agent targeting all three runtimes: native on claude, emulated on codex
	// AND gemini (capability matrix §6). support/outputs cover every targeted
	// runtime; emulated ones carry a fidelity note, the native one does not (CC-4).
	cat := &model.Catalog{Bundles: []*model.Bundle{{
		Name: "stark-review", Version: "0.1.0", Description: "d", Owner: model.Owner{Name: "E"},
		Runtimes: model.AllRuntimes(),
		Artifacts: []*model.Artifact{{
			Name: "red-team", Type: model.TypeAgent, Description: "adversary", Version: "0.1.0",
			Runtimes: model.AllRuntimes(), Raw: map[string]any{}, Body: "Attack the design.\n",
		}},
	}}}
	idx, details, err := Build(cat)
	if err != nil {
		t.Fatal(err)
	}
	da := details["stark-review"].Artifacts[0]

	for _, rt := range []string{"claude", "codex", "gemini"} {
		if da.Support[rt] == "" {
			t.Fatalf("detail support missing for runtime %q (claude-only regression)", rt)
		}
		if len(da.Outputs[rt]) == 0 || da.Outputs[rt][0].Path == "" || da.Outputs[rt][0].Kind == "" {
			t.Fatalf("detail outputs missing/invalid for %q: %+v", rt, da.Outputs[rt])
		}
	}
	if da.Support["claude"] != string(model.SupportNative) {
		t.Fatalf("claude agent should be native, got %q", da.Support["claude"])
	}
	if da.Support["codex"] != string(model.SupportEmulated) || da.Support["gemini"] != string(model.SupportEmulated) {
		t.Fatalf("codex/gemini agent should be emulated: %+v", da.Support)
	}
	if da.FidelityNotes["codex"] == "" || da.FidelityNotes["gemini"] == "" {
		t.Fatalf("emulated runtimes must carry a fidelity note: %+v", da.FidelityNotes)
	}
	if _, ok := da.FidelityNotes["claude"]; ok {
		t.Fatal("claude agent (native) must NOT carry a fidelityNote")
	}
	// lean entry support also covers all three.
	e := idx.Artifacts[0]
	for _, rt := range []string{"claude", "codex", "gemini"} {
		if e.Support[rt] == "" {
			t.Fatalf("lean entry support missing for %q", rt)
		}
	}
}

// TestCommittedIndexSupportFullyPopulated asserts the committed repo-root
// index.json has a non-empty support level for EVERY targeted runtime an artifact
// opts into (CC-4), guarding against a claude-only regression.
func TestCommittedIndexSupportFullyPopulated(t *testing.T) {
	raw, err := os.ReadFile("../../../index.json")
	if err != nil {
		t.Fatal(err)
	}
	var idx struct {
		Artifacts []struct {
			Name     string            `json:"name"`
			Runtimes []string          `json:"runtimes"`
			Support  map[string]string `json:"support"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatal(err)
	}
	for _, a := range idx.Artifacts {
		for _, rt := range a.Runtimes {
			if a.Support[rt] == "" {
				t.Fatalf("artifact %q missing support for targeted runtime %q", a.Name, rt)
			}
		}
	}
}
