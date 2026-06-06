package build

import (
	"strings"
	"testing"

	"github.com/GetEvinced/stark-marketplace/engine/internal/load"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

func TestBuildProducesClaudeTreeAndIndex(t *testing.T) {
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Build(cat)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Files["index.json"]; !ok {
		t.Fatal("index.json not produced")
	}
	if _, ok := out.Files["bundles/stark-gh.json"]; !ok {
		t.Fatal("bundle detail not produced")
	}
	foundClaude := false
	for p := range out.Files {
		if strings.HasPrefix(p, "dist/claude/stark-gh/") {
			foundClaude = true
		}
	}
	if !foundClaude {
		t.Fatalf("no dist/claude files; got %v", keys(out.Files))
	}
	// divergence budget present (seed has 0 diverged)
	if !strings.Contains(out.DivergenceBudget, "diverged") {
		t.Fatalf("budget = %q", out.DivergenceBudget)
	}
}

func keys(m map[string][]byte) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestDivergenceBudgetCountsDiverged(t *testing.T) {
	// A claude artifact with an annotated full-body override is author-divergence:
	// merge.Resolve marks it Diverged, claude.foldFindings prefixes "diverged: ", and
	// build.Build counts it. Locks that whole string-coupled chain at a nonzero count.
	cat := &model.Catalog{Bundles: []*model.Bundle{{
		Name: "demo", Version: "0.1.0", Description: "d", Owner: model.Owner{Name: "E"},
		Runtimes: []model.Runtime{model.RuntimeClaude},
		Artifacts: []*model.Artifact{{
			Name: "rev", Type: model.TypeSkill, Description: "s", Version: "0.1.0",
			Runtimes: []model.Runtime{model.RuntimeClaude}, Raw: map[string]any{}, Body: "base\n",
			Overrides: map[model.Runtime]model.Override{
				model.RuntimeClaude: {Body: "# diverged: needs a claude-specific body\nClaude body\n"},
			},
		}},
	}}}
	out, err := Build(cat)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.DivergenceBudget, "diverged 1 /") {
		t.Fatalf("want budget to count 1 diverged, got %q", out.DivergenceBudget)
	}
}
