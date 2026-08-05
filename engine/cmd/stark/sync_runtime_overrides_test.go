package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestRunSyncSeparatesCodexArtifactAndSupportOverrides(t *testing.T) {
	repoRoot := t.TempDir()
	catalogDir := filepath.Join(repoRoot, "catalog")
	from := t.TempDir()

	writeSyncFixtureFile(t, filepath.Join(catalogDir, "demo", "bundle.yaml"), `name: demo
version: 1.2.3
description: Demo bundle.
owner: {name: Example}
runtimes: [claude, codex]
skills: [demo-skill]
`)
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
}
