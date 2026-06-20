package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/aggregate"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/install"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/installplan"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
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
// This is the live-surface proof that `stark install` writes REAL payloads (not fakes) for
// codex/gemini/claude — the artifacts the marketplace actually ships.
func TestRealAdapterRendersCommittedCatalog(t *testing.T) {
	root := repoRoot(t)
	idx, err := indexio.LoadIndex(filepath.Join(root, "index.json"))
	if err != nil {
		t.Skipf("committed index.json not present (%v) — skipping live-catalog test", err)
	}
	bundles := filepath.Join(root, "bundles")
	ad := realAdapter(filepath.Join(root, "catalog"))

	t.Run("codex", func(t *testing.T) {
		dest := t.TempDir()
		os.WriteFile(filepath.Join(dest, "config.toml"), []byte("# mine\nmodel = \"gpt\"\n"), 0o644)
		p, err := installplan.Compute(idx, bundles, ad, "stark-gh", "", model.TypeCommand, model.RuntimeCodex)
		if err != nil {
			t.Fatal(err)
		}
		if !p.Consent.Required {
			t.Fatal("stark-gh closure has an mcp — consent must be required")
		}
		res, err := install.Install(dest, p, install.Options{})
		if err != nil {
			t.Fatalf("install: %v", err)
		}
		// real command body (codex !claude runtime variant), not a fake placeholder
		skill, _ := os.ReadFile(filepath.Join(dest, ".agents/skills/pr-open/SKILL.md"))
		if !strings.Contains(string(skill), "Open or update a GitHub pull request") {
			t.Fatalf("SKILL.md missing real body:\n%s", skill)
		}
		// real MCP merge: keyed table + env subtable + placeholder (never the secret value)
		cfg, _ := os.ReadFile(filepath.Join(dest, "config.toml"))
		s := string(cfg)
		for _, want := range []string{"# mine", "[mcp_servers.gh]", "command = 'node'", "[mcp_servers.gh.env]", "${GITHUB_TOKEN}"} {
			if !strings.Contains(s, want) {
				t.Fatalf("config.toml missing %q:\n%s", want, s)
			}
		}
		if strings.Contains(s, "stark-gh-token") {
			t.Fatalf("codex must use the env placeholder, never the secretRef value:\n%s", s)
		}
		// idempotent
		first, _ := os.ReadFile(filepath.Join(dest, "config.toml"))
		if _, err := install.Install(dest, p, install.Options{}); err != nil {
			t.Fatalf("re-install: %v", err)
		}
		second, _ := os.ReadFile(filepath.Join(dest, "config.toml"))
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
		after, _ := os.ReadFile(filepath.Join(dest, "config.toml"))
		if !strings.Contains(string(after), "# mine") || strings.Contains(string(after), "mcp_servers.gh") {
			t.Fatalf("remove did not excise precisely:\n%s", after)
		}
	})

	t.Run("gemini", func(t *testing.T) {
		dest := t.TempDir()
		os.WriteFile(filepath.Join(dest, "settings.json"), []byte(`{"theme":"dark"}`), 0o644)
		p, err := installplan.Compute(idx, bundles, ad, "stark-gh", "gh", model.TypeMCP, model.RuntimeGemini)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := install.Install(dest, p, install.Options{}); err != nil {
			t.Fatalf("install: %v", err)
		}
		got, _ := os.ReadFile(filepath.Join(dest, "settings.json"))
		s := string(got)
		if !strings.Contains(s, `"theme": "dark"`) || !strings.Contains(s, `"gh"`) || !strings.Contains(s, "${GITHUB_TOKEN}") {
			t.Fatalf("gemini settings.json merge wrong:\n%s", s)
		}
	})

	t.Run("claude", func(t *testing.T) {
		dest := t.TempDir()
		p, err := installplan.Compute(idx, bundles, ad, "stark-gh", "gh", model.TypeMCP, model.RuntimeClaude)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := install.Install(dest, p, install.Options{}); err != nil {
			t.Fatalf("install: %v", err)
		}
		got, _ := os.ReadFile(filepath.Join(dest, ".mcp.json"))
		if !strings.Contains(string(got), `"gh"`) || !strings.Contains(string(got), `"command": "node"`) {
			t.Fatalf("claude .mcp.json merge wrong:\n%s", got)
		}
	})
}
