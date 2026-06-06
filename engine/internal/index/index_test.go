package index

import (
	"encoding/json"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestBuildLeanIndex(t *testing.T) {
	cat := &model.Catalog{Bundles: []*model.Bundle{{
		Name: "demo", Version: "0.1.0", Category: "examples", Maturity: model.MaturityBeta,
		Runtimes: []model.Runtime{model.RuntimeClaude},
		Artifacts: []*model.Artifact{
			{Name: "rev", Type: model.TypeSkill, Description: "review skill", Version: "0.2.0", Tags: []string{"x"},
				Category: "examples", Maturity: model.MaturityStable,
				Runtimes: []model.Runtime{model.RuntimeClaude}, Body: "b\n"},
		},
	}}}
	idx, details := Build(cat)
	if idx.SchemaVersion == 0 {
		t.Fatal("schemaVersion must be set")
	}
	if len(idx.Artifacts) != 1 {
		t.Fatalf("want 1 entry, got %d", len(idx.Artifacts))
	}
	e := idx.Artifacts[0]
	if e.Name != "rev" || e.Bundle != "demo" || e.Version != "0.2.0" {
		t.Fatalf("entry = %+v", e)
	}
	if e.Description != "review skill" {
		t.Fatalf("description not carried into lean entry: %+v", e)
	}
	if e.Support["claude"] != string(model.SupportNative) {
		t.Fatalf("claude support badge missing: %+v", e.Support)
	}
	if e.Digest == "" {
		t.Fatal("digest missing")
	}
	// CC-2: per-entry runtimes for runtime-filtered search without fetching detail.
	if len(e.Runtimes) != 1 || e.Runtimes[0] != "claude" {
		t.Fatalf("lean entry runtimes missing/wrong: %+v", e.Runtimes)
	}
	// CC-2: top-level provenance hook.
	if idx.GeneratedBy.AdapterVersions["claude"] == "" {
		t.Fatalf("generatedBy.adapterVersions[claude] missing: %+v", idx.GeneratedBy)
	}
	d, ok := details["demo"]
	if !ok {
		t.Fatal("missing bundle detail")
	}
	// CC-3 structured detail: per-artifact support/requires/diverged/outputs/fidelityNotes.
	if len(d.Artifacts) != 1 {
		t.Fatalf("detail artifacts = %d", len(d.Artifacts))
	}
	da := d.Artifacts[0]
	if da.Support["claude"] != string(model.SupportNative) {
		t.Fatalf("detail claude support missing: %+v", da.Support)
	}
	outs, ok := da.Outputs["claude"]
	if !ok || len(outs) == 0 {
		t.Fatalf("detail claude outputs missing: %+v", da.Outputs)
	}
	if outs[0].Path != "skills/rev/SKILL.md" || outs[0].Kind != "file" {
		t.Fatalf("detail output[0] = %+v", outs[0])
	}
	// codex/gemini are present-but-empty in slice 2 (filled by plan 03).
	if _, ok := da.Outputs["codex"]; ok {
		t.Fatal("codex outputs must be absent until plan 03")
	}
	// lean index must be marshalable and contain schemaVersion
	b, _ := json.Marshal(idx)
	if len(b) == 0 {
		t.Fatal("index not marshalable")
	}
}
