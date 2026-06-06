package importer

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestDefaultsAppliedAndNoted(t *testing.T) {
	a := &model.Artifact{Name: "x", Type: model.TypeSkill, Description: "d"}
	res := &ImportResult{}
	applyArtifactDefaults(a, res, "demo/skill/x")

	if a.Version != defaultVersion {
		t.Fatalf("version = %q, want %q", a.Version, defaultVersion)
	}
	if a.Maturity != model.MaturityBeta {
		t.Fatalf("maturity = %q, want beta", a.Maturity)
	}
	if len(a.Runtimes) != 1 || a.Runtimes[0] != model.RuntimeClaude {
		t.Fatalf("runtimes = %+v, want [claude]", a.Runtimes)
	}
	// every defaulted field must be recorded for the human checklist
	if len(res.Notes) < 3 {
		t.Fatalf("want >=3 metadata notes, got %d: %+v", len(res.Notes), res.Notes)
	}
}
