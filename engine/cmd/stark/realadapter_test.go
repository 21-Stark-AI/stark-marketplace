package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/aggregate"
	"github.com/21StarkCom/bifrost/engine/internal/indexio"
	"github.com/21StarkCom/bifrost/engine/internal/install"
	"github.com/21StarkCom/bifrost/engine/internal/installplan"
	"github.com/21StarkCom/bifrost/engine/internal/model"
)

// The real adapter renders sentinel emulation blocks with digest-bearing markers
// (`<!-- stark:begin id@<digest> -->`). sentinelBody must recover the CLEAN inner body so
// install.MergeSentinel wraps it exactly once — otherwise the markers leak and a second install
// double-wraps and corrupts GEMINI.md/AGENTS.md.
func TestSentinelBodyStripsRenderMarkersNoDoubleWrap(t *testing.T) {
	rendered := aggregate.Merge([]aggregate.Section{{Bundle: "multi", Name: "agentmd", Content: "agent role line\n"}})
	if !strings.Contains(rendered, "@") {
		t.Fatalf("precondition: render markers should carry a digest:\n%s", rendered)
	}
	body, err := sentinelBody(rendered, "multi/agentmd")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "stark:begin") || strings.Contains(body, "stark:end") {
		t.Fatalf("markers leaked into stripped body: %q", body)
	}
	if !strings.Contains(body, "agent role line") {
		t.Fatalf("body content lost: %q", body)
	}
	// install wraps the clean body exactly once, and a re-merge is idempotent
	once, _, err := install.MergeSentinel(nil, "multi/agentmd", body)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(once), "stark:begin multi/agentmd"); n != 1 {
		t.Fatalf("expected exactly one begin marker, got %d:\n%s", n, once)
	}
	twice, _, _ := install.MergeSentinel(once, "multi/agentmd", body)
	if string(once) != string(twice) {
		t.Fatalf("sentinel re-merge not idempotent")
	}
	if _, err := sentinelBody(rendered, "multi/missing"); err == nil {
		t.Fatal("sentinelBody must error when the section id is absent")
	}
}

// TestRealAdapterRendersCommittedCatalog exercises the PRODUCTION adapter (catalogAdapter):
// it renders slice-03's runtime targets in-memory from the committed catalog and applies them.
// This is the live-surface proof that `stark install` writes real Codex payloads
// (not fakes) for the artifacts the marketplace actually ships.
func TestRealAdapterRendersCommittedCatalog(t *testing.T) {
	root := repoRoot(t)
	idx, err := indexio.LoadIndex(filepath.Join(root, "index.json"))
	if err != nil {
		t.Skipf("committed index.json not present (%v) — skipping live-catalog test", err)
	}
	bundles := filepath.Join(root, "bundles")
	ad := realAdapter(filepath.Join(root, "catalog"),
		filepath.Join(root, "vendor", "stark-skills"), filepath.Join(root, "vendor", "plugins"))

	t.Run("codex", func(t *testing.T) {
		// stark-gh, the only bundle that shipped commands, was retired in STARK-2211,
		// so the live command-rendering subject is gone. stark-housekeeping is a
		// disable-model-invocation skill, which preserves the explicit-only
		// (allow_implicit_invocation: false) assertion this test exists to guard.
		dest := t.TempDir()
		p, err := installplan.Compute(idx, bundles, ad, "stark-ops", "", model.TypeSkill, model.RuntimeCodex)
		if err != nil {
			t.Fatal(err)
		}
		if !p.Consent.Required {
			t.Fatal("stark-ops ships executable vendored tools — consent must be required")
		}
		res, err := install.Install(dest, p, install.Options{})
		if err != nil {
			t.Fatalf("install: %v", err)
		}
		// real skill body (codex runtime variant), not a fake placeholder
		skill, _ := os.ReadFile(filepath.Join(dest, ".agents/skills/stark-housekeeping/SKILL.md"))
		if !strings.Contains(string(skill), "Usage: stark-housekeeping [--dry-run]") {
			t.Fatalf("SKILL.md missing real body:\n%s", skill)
		}
		metadata, _ := os.ReadFile(filepath.Join(dest, ".agents/skills/stark-housekeeping/agents/openai.yaml"))
		if !strings.Contains(string(metadata), "allow_implicit_invocation: false") {
			t.Fatalf("skill lost its explicit-only boundary:\n%s", metadata)
		}
		// idempotent
		first := append([]byte(nil), skill...)
		if _, err := install.Install(dest, p, install.Options{}); err != nil {
			t.Fatalf("re-install: %v", err)
		}
		second, _ := os.ReadFile(filepath.Join(dest, ".agents/skills/stark-housekeeping/SKILL.md"))
		if string(first) != string(second) {
			t.Fatalf("real install not idempotent")
		}
		// doctor clean, then precise removal
		if rep, _ := install.Doctor(dest, res.ManifestPath); len(rep.Broken) != 0 {
			t.Fatalf("doctor broken: %+v", rep.Broken)
		}
		if err := install.Remove(dest, res.ManifestPath); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dest, ".agents/skills/stark-housekeeping/SKILL.md")); !os.IsNotExist(err) {
			t.Fatalf("remove left the managed skill behind: %v", err)
		}
	})
}
