package index

import (
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// Spec §7.4 requires agent.tools to be "surfaced in the index"; the surfacing home is the CC-3
// bundle detail (the install-consent surface, §9.3). Assert the grant list lands on DetailEntry.
func TestBundleDetailSurfacesAgentTools(t *testing.T) {
	cat := &model.Catalog{Bundles: []*model.Bundle{{
		Name: "demo", Version: "0.1.0", Runtimes: []model.Runtime{model.RuntimeClaude},
		Artifacts: []*model.Artifact{{
			Name: "helper", Type: model.TypeAgent, Bundle: "demo", Version: "0.1.0",
			Description: "an agent", Body: "do the thing\n",
			Runtimes: []model.Runtime{model.RuntimeClaude},
			Tools:    []string{"Read", "Bash"},
		}},
	}}}
	_, details, err := Build(cat)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	d, ok := details["demo"]
	if !ok || len(d.Artifacts) != 1 {
		t.Fatalf("expected one detail artifact, got %+v", details)
	}
	got := d.Artifacts[0].Tools
	if len(got) != 2 || got[0] != "Read" || got[1] != "Bash" {
		t.Fatalf("agent tools not surfaced in bundle detail: %+v", got)
	}
}

// A non-agent artifact carries no tools, so the omitempty field stays empty (no detail churn).
func TestBundleDetailNoToolsForNonAgent(t *testing.T) {
	cat := &model.Catalog{Bundles: []*model.Bundle{{
		Name: "demo", Version: "0.1.0", Runtimes: []model.Runtime{model.RuntimeClaude},
		Artifacts: []*model.Artifact{{
			Name: "do-x", Type: model.TypeCommand, Bundle: "demo", Version: "0.1.0",
			Description: "a command", Body: "run\n",
			Runtimes: []model.Runtime{model.RuntimeClaude},
		}},
	}}}
	_, details, err := Build(cat)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := details["demo"].Artifacts[0].Tools; len(got) != 0 {
		t.Fatalf("non-agent must have no surfaced tools, got %+v", got)
	}
}
