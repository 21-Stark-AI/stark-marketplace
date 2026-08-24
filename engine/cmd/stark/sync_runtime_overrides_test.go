package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/build"
	"github.com/21StarkCom/bifrost/engine/internal/load"
	"github.com/21StarkCom/bifrost/engine/internal/marketplace"
)

func writeSyncFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMinimalSyncVendorInputs(t *testing.T, from string) {
	t.Helper()
	writeSyncFixtureFile(t, filepath.Join(from, "tools", "runtime.ts"), "export {}\n")
	writeSyncFixtureFile(t, filepath.Join(from, "global", "prompts", "prompt.md"), "prompt\n")
	writeSyncFixtureFile(t, filepath.Join(from, "standards", "help.md"), "help\n")
	writeSyncFixtureFile(t, filepath.Join(from, "scripts", "run.sh"), "#!/bin/sh\n")
	writeSyncFixtureFile(t, filepath.Join(from, "data", "persona", "roster.md"), "roster\n")
	writeSyncFixtureFile(t, filepath.Join(from, "global", "config.json"), "{}\n")
	writeSyncFixtureFile(t, filepath.Join(from, "global", "forge_heuristics.json"), "{}\n")
}

func TestRunSyncAllowsCuratedMCPOnlyBundle(t *testing.T) {
	repoRoot := t.TempDir()
	catalogDir := filepath.Join(repoRoot, "catalog")
	from := t.TempDir()

	writeSyncFixtureFile(t, filepath.Join(catalogDir, "brain", "bundle.yaml"), `name: brain
version: 1.0.0
description: Brain bundle.
owner: {name: Example}
runtimes: [claude, codex]
`)
	writeSyncFixtureFile(t, filepath.Join(catalogDir, "brain", "mcp", "brain.yaml"), `name: brain
type: mcp
description: Brain MCP.
version: 1.0.0
mcp:
  transport: stdio
  command: brain
  args: [mcp]
`)
	writeSyncFixtureFile(t, filepath.Join(catalogDir, "brain", "skills", "stale.md"), `---
name: stale
type: skill
description: Stale generated skill.
version: 1.0.0
---
Stale.
`)
	writeMinimalSyncVendorInputs(t, from)

	if code := runSync(from, catalogDir, repoRoot, false); code != 0 {
		t.Fatalf("runSync exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(catalogDir, "brain", "skills", "stale.md")); !os.IsNotExist(err) {
		t.Fatalf("stale generated skill survived MCP-only sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(catalogDir, "brain", "mcp", "brain.yaml")); err != nil {
		t.Fatalf("curated MCP was removed: %v", err)
	}
}

func TestRunSyncRejectsBundleWithoutGeneratedOrCuratedArtifacts(t *testing.T) {
	repoRoot := t.TempDir()
	catalogDir := filepath.Join(repoRoot, "catalog")
	from := t.TempDir()

	writeSyncFixtureFile(t, filepath.Join(catalogDir, "empty", "bundle.yaml"), `name: empty
version: 1.0.0
description: Empty bundle.
owner: {name: Example}
runtimes: [claude, codex]
`)
	writeMinimalSyncVendorInputs(t, from)

	if code := runSync(from, catalogDir, repoRoot, false); code == 0 {
		t.Fatal("runSync accepted a bundle with no generated source or curated artifacts")
	}
}

func TestRunSyncSeparatesCodexArtifactAndSupportOverrides(t *testing.T) {
	repoRoot := t.TempDir()
	catalogDir := filepath.Join(repoRoot, "catalog")
	from := t.TempDir()

	bundleYAML := `name: demo
version: 1.2.3
description: Demo bundle.
owner: {name: Example}
runtimes: [claude, codex]
skills: [demo-skill]
`
	writeSyncFixtureFile(t, filepath.Join(catalogDir, "demo", "bundle.yaml"), bundleYAML)
	writeSyncFixtureFile(t, filepath.Join(from, "skill", "demo-skill", "SKILL.md"), `---
name: demo-skill
description: Claude description.
argument-hint: "[old]"
disable-model-invocation: true
---
Claude body.
`)
	writeSyncFixtureFile(t, filepath.Join(from, "runtime-overrides", "codex", "skill", "demo-skill", "SKILL.md"), `---
name: demo-skill
description: Codex description.
argument-hint: "[new]"
disable-model-invocation: true
---
Codex body.

---

Codex-only tail after a Markdown horizontal rule.
`)
	writeSyncFixtureFile(t, filepath.Join(from, "runtime-overrides", "codex", "standards", "help.md"), "Codex help.\n")
	writeSyncFixtureFile(t, filepath.Join(from, "runtime-overrides", "codex", "global", "config.json"), `{"source":"codex-global"}`+"\n")
	writeSyncFixtureFile(t, filepath.Join(from, "plugins", "demo", "config.json"), `{"source":"plugin"}`+"\n")

	// Minimal shared snapshot inputs required by VendorSnapshot.
	writeSyncFixtureFile(t, filepath.Join(from, "tools", "runtime.ts"), "export {}\n")
	writeSyncFixtureFile(t, filepath.Join(from, "global", "prompts", "prompt.md"), "prompt\n")
	writeSyncFixtureFile(t, filepath.Join(from, "standards", "help.md"), "Claude help.\n")
	writeSyncFixtureFile(t, filepath.Join(from, "scripts", "run.sh"), "#!/bin/sh\n")
	writeSyncFixtureFile(t, filepath.Join(from, "data", "persona", "roster.md"), "roster\n")
	writeSyncFixtureFile(t, filepath.Join(from, "global", "config.json"), "{}\n")
	writeSyncFixtureFile(t, filepath.Join(from, "global", "forge_heuristics.json"), "{}\n")

	if code := runSync(from, catalogDir, repoRoot, false); code != 0 {
		t.Fatalf("runSync exit = %d", code)
	}

	catalogSkill, err := os.ReadFile(filepath.Join(catalogDir, "demo", "skills", "demo-skill.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(catalogSkill)
	if !strings.Contains(text, "Usage: demo-skill [old]\n\nClaude body.") {
		t.Fatalf("canonical body changed:\n%s", text)
	}
	if !strings.Contains(text, "overrides:\n  codex:") || !strings.Contains(text, "Codex body.") {
		t.Fatalf("Codex artifact override missing:\n%s", text)
	}

	codexSupport := filepath.Join(repoRoot, "vendor", "runtime-overrides", "codex", "demo", "standards", "help.md")
	if got, err := os.ReadFile(codexSupport); err != nil || string(got) != "Codex help.\n" {
		t.Fatalf("Codex support snapshot = %q, %v", got, err)
	}
	pluginConfig := filepath.Join(repoRoot, "vendor", "plugins", "demo", "config.json")
	if got, err := os.ReadFile(pluginConfig); err != nil || string(got) != `{"source":"plugin"}`+"\n" {
		t.Fatalf("plugin config snapshot = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "vendor", "runtime-overrides", "codex", "demo", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("Codex global config must not mask plugin-specific config: %v", err)
	}
	for _, leaked := range []string{
		filepath.Join(repoRoot, "vendor", "runtime-overrides", "codex", "demo", "skills", "demo-skill", "SKILL.md"),
		filepath.Join(repoRoot, "vendor", "stark-skills", "runtime-overrides", "codex", "standards", "help.md"),
		filepath.Join(repoRoot, "vendor", "plugins", "demo", "standards", "help.md"),
		filepath.Join(repoRoot, "dist", "claude", "demo", "standards", "help.md"),
	} {
		if _, err := os.Stat(leaked); !os.IsNotExist(err) {
			t.Errorf("runtime override leaked to %s", leaked)
		}
	}

	// Exercise the complete source -> sync serialization -> catalog loader -> build
	// round trip. A Codex override is allowed to contain ordinary Markdown rules;
	// adding it must leave every Claude package byte and the Claude marketplace
	// manifest identical to a sync from the same canonical source without overrides.
	withOverrideCatalog, err := load.Load(catalogDir)
	if err != nil {
		t.Fatal(err)
	}
	withOverride, err := build.Build(withOverrideCatalog, build.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(from, "runtime-overrides")); err != nil {
		t.Fatal(err)
	}
	controlRoot := t.TempDir()
	controlCatalogDir := filepath.Join(controlRoot, "catalog")
	writeSyncFixtureFile(t, filepath.Join(controlCatalogDir, "demo", "bundle.yaml"), bundleYAML)
	if code := runSync(from, controlCatalogDir, controlRoot, false); code != 0 {
		t.Fatalf("control runSync exit = %d", code)
	}
	controlCatalog, err := load.Load(controlCatalogDir)
	if err != nil {
		t.Fatal(err)
	}
	control, err := build.Build(controlCatalog, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertClaudeOwnedBytesEqual(t, control.Files, withOverride.Files)
}

func assertClaudeOwnedBytesEqual(t *testing.T, want, got map[string][]byte) {
	t.Helper()
	isClaudeOwned := func(path string) bool {
		return strings.HasPrefix(path, "dist/claude/") || path == marketplace.ManifestRelPath
	}
	for path, wantBody := range want {
		if !isClaudeOwned(path) {
			continue
		}
		gotBody, ok := got[path]
		if !ok {
			t.Fatalf("Codex override removed Claude-owned output %s", path)
		}
		if !bytes.Equal(gotBody, wantBody) {
			t.Fatalf("Codex override changed Claude-owned output %s\n--- want ---\n%s\n--- got ---\n%s", path, wantBody, gotBody)
		}
	}
	for path := range got {
		if isClaudeOwned(path) {
			if _, ok := want[path]; !ok {
				t.Fatalf("Codex override added Claude-owned output %s", path)
			}
		}
	}
}
