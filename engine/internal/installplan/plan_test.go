package installplan

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func loadFx(t *testing.T) (*indexio.Index, string) {
	t.Helper()
	idx, err := indexio.LoadIndex("testdata/index.json")
	if err != nil {
		t.Fatal(err)
	}
	return idx, "testdata/bundles"
}

func TestComputeDAGOrderDepsFirst(t *testing.T) {
	idx, bdir := loadFx(t)
	fa := NewFakeAdapter(nil)
	p, err := Compute(idx, bdir, fa, "rev", "review", model.TypeCommand, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	last := p.Steps[len(p.Steps)-1]
	if last.Name != "review" {
		t.Fatalf("root must be last, got %s", last.Name)
	}
	if len(p.Steps) != 3 {
		t.Fatalf("want 3 steps, got %d", len(p.Steps))
	}
}

func TestComputeSkipsNonTargetingArtifacts(t *testing.T) {
	idx, bdir := loadFx(t)
	fa := NewFakeAdapter(nil)
	// session is unsupported on gemini; installing the bundle on gemini must skip it.
	p, err := Compute(idx, bdir, fa, "rev", "", model.TypeCommand, model.RuntimeGemini)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range p.Skipped {
		if s == "rev/session" {
			found = true
		}
	}
	if !found {
		t.Fatalf("session should be skipped on gemini: %+v", p.Skipped)
	}
}

func TestConsentFlagsMCP(t *testing.T) {
	idx, bdir := loadFx(t)
	fa := NewFakeAdapter(nil)
	p, _ := Compute(idx, bdir, fa, "rev", "review", model.TypeCommand, model.RuntimeCodex)
	if !p.Consent.Required {
		t.Fatal("mcp dep (bq) must require consent")
	}
	if len(p.Consent.MCPCommands) == 0 {
		t.Fatal("consent must list the mcp command")
	}
}

func TestClosureRefs(t *testing.T) {
	idx, bdir := loadFx(t)
	refs, err := ClosureRefs(idx, bdir, "rev", "review", model.TypeCommand)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 { // session + bq
		t.Fatalf("closure = %v", refs)
	}
}
