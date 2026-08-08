package importer

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/21StarkCom/bifrost/engine/internal/adapter/claude"
	"github.com/21StarkCom/bifrost/engine/internal/load"
	"github.com/21StarkCom/bifrost/engine/internal/merge"
	"github.com/21StarkCom/bifrost/engine/internal/model"
)

func TestImportAttachesSourceOwnedCodexSkillOverride(t *testing.T) {
	from := t.TempDir()
	writeFile(t, filepath.Join(from, "skill", "demo", "SKILL.md"), `---
name: demo
description: Claude description.
argument-hint: "[old]"
disable-model-invocation: true
model: opus
---

Claude body.
`)
	writeFile(t, filepath.Join(from, "runtime-overrides", "codex", "skill", "demo", "SKILL.md"), `---
name: demo
description: Codex description.
argument-hint: "[new]"
disable-model-invocation: true
model: codex-placeholder
---

Codex body.
---
Codex tail.
`)

	res, err := ImportForGenerator(from, "demo-bundle", []string{"demo"})
	if err != nil {
		t.Fatal(err)
	}
	a := findArtifact(res.Bundle, "demo")
	if a == nil {
		t.Fatal("demo skill not imported")
	}
	ov, ok := a.Overrides[model.RuntimeCodex]
	if !ok {
		t.Fatal("Codex override not attached")
	}
	if got := ov.Fields["description"]; got != "Codex description." {
		t.Fatalf("override description = %q", got)
	}
	if got := ov.Fields["argument-hint"]; got != "[new]" {
		t.Fatalf("override argument-hint = %q", got)
	}
	if !strings.HasPrefix(ov.Body, "# diverged: "+codexOverrideReason+"\nCodex body.") {
		t.Fatalf("override body = %q", ov.Body)
	}

	files, err := ArtifactFiles(res)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(files["skills/demo.md"])
	if !strings.Contains(serialized, "Usage: demo [old]\n\nClaude body.") {
		t.Fatalf("canonical Claude body/hint changed:\n%s", serialized)
	}
	if !strings.Contains(serialized, "overrides:\n  codex:") ||
		!strings.Contains(serialized, "Codex body.") ||
		!strings.Contains(serialized, "      ---\n      Codex tail.") {
		t.Fatalf("serialized Codex override missing:\n%s", serialized)
	}

	// Exercise the production sync -> serialize -> catalog load path. The
	// indented Markdown rule in overrides.codex.body must remain inside YAML
	// frontmatter rather than being mistaken for its closing delimiter.
	catalogDir := t.TempDir()
	writeFile(t, filepath.Join(catalogDir, "demo-bundle", "bundle.yaml"), `name: demo-bundle
version: 0.1.0
description: Demo bundle.
owner:
  name: Demo
runtimes:
  - claude
  - codex
`)
	writeFile(t, filepath.Join(catalogDir, "demo-bundle", "skills", "demo.md"), serialized)
	cat, err := load.Load(catalogDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Bundles) != 1 || len(cat.Bundles[0].Artifacts) != 1 {
		t.Fatalf("loaded catalog = %+v", cat)
	}
	loaded := cat.Bundles[0].Artifacts[0]
	resolved, findings, err := merge.Resolve(loaded, model.RuntimeCodex)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Body != "Codex body.\n---\nCodex tail.\n" || resolved.Frontmatter["description"] != "Codex description." {
		t.Fatalf("resolved override = %+v body=%q", resolved.Frontmatter, resolved.Body)
	}
	if !findings.Diverged || findings.DivergedReason != codexOverrideReason {
		t.Fatalf("divergence findings = %+v", findings)
	}

	// The same loaded catalog artifact must remain byte-faithful on Claude: the
	// Codex override is present in metadata but is never selected by that target.
	claudeFiles, _, err := claude.New().Render(&model.Bundle{
		Name:      "demo-bundle",
		Artifacts: []*model.Artifact{loaded},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(claudeFiles) != 2 {
		t.Fatalf("Claude files = %+v", claudeFiles)
	}
	var claudeSkill string
	for _, file := range claudeFiles {
		if file.Path == "skills/demo/SKILL.md" {
			claudeSkill = string(file.Content)
		}
	}
	wantClaude := `---
name: demo
description: Claude description.
disable-model-invocation: true
model: opus
---
Usage: demo [old]

Claude body.
`
	if claudeSkill != wantClaude {
		t.Fatalf("Claude output changed:\n--- got ---\n%s--- want ---\n%s", claudeSkill, wantClaude)
	}
	for _, fragment := range []string{"# diverged:", "Codex body.", "Codex tail."} {
		if strings.Contains(claudeSkill, fragment) {
			t.Fatalf("Codex override fragment %q affected Claude output:\n%s", fragment, claudeSkill)
		}
	}
}

func TestImportAttachesCodexCommandOverride(t *testing.T) {
	from := t.TempDir()
	writeFile(t, filepath.Join(from, "plugins", "demo-gh", "commands", "cleanup.md"), `---
name: cleanup
description: Claude cleanup.
argument-hint: "[--dry-run]"
allowed-tools: Bash
---
Claude command body.
`)
	writeFile(t, filepath.Join(from, "runtime-overrides", "codex", "plugins", "demo-gh", "commands", "cleanup.md"), `---
name: cleanup
description: Codex cleanup.
argument-hint: "[--dry-run]"
allowed-tools: Bash
---
Codex command body.
`)

	res, err := ImportForGenerator(from, "demo-gh", nil)
	if err != nil {
		t.Fatal(err)
	}
	a := findArtifact(res.Bundle, "cleanup")
	if a == nil || a.Overrides[model.RuntimeCodex].Body == "" {
		t.Fatalf("command override missing: %+v", a)
	}
}

func TestImportRejectsRuntimeOverrideIdentityDrift(t *testing.T) {
	from := t.TempDir()
	writeFile(t, filepath.Join(from, "skill", "demo", "SKILL.md"), `---
name: demo
description: Claude description.
---
Claude body.
`)
	writeFile(t, filepath.Join(from, "runtime-overrides", "codex", "skill", "demo", "SKILL.md"), `---
name: another-skill
description: Codex description.
---
Codex body.
`)

	_, err := ImportForGenerator(from, "demo-bundle", []string{"demo"})
	if err == nil || !strings.Contains(err.Error(), `name "another-skill" != canonical artifact "demo"`) {
		t.Fatalf("identity drift error = %v", err)
	}
}

func TestImportFailsClosedWhenEnabledOverlayIsMissingArtifact(t *testing.T) {
	from := t.TempDir()
	writeFile(t, filepath.Join(from, "skill", "demo", "SKILL.md"), `---
name: demo
description: Claude description.
---
Claude body.
`)
	// Any Codex override root enables the production contract for every imported
	// skill/command; a partial overlay must not silently fall back to Claude text.
	writeFile(t, filepath.Join(from, "runtime-overrides", "codex", "standards", "help.md"), "help\n")

	_, err := ImportForGenerator(from, "demo-bundle", []string{"demo"})
	if err == nil || !strings.Contains(err.Error(), "Codex runtime override is enabled") {
		t.Fatalf("missing overlay error = %v", err)
	}
}

func TestClaudeOnlySkillIsExemptFromCodexOverlayRequirement(t *testing.T) {
	from := t.TempDir()
	// `runtimes: [claude]` is the author's declaration that no Codex variant
	// exists on purpose — the enabled-overlay contract must skip it, not fail.
	writeFile(t, filepath.Join(from, "skill", "demo", "SKILL.md"), `---
name: demo
description: Claude description.
runtimes:
  - claude
---
Claude body.
`)
	writeFile(t, filepath.Join(from, "runtime-overrides", "codex", "standards", "help.md"), "help\n")

	res, err := ImportForGenerator(from, "demo-bundle", []string{"demo"})
	if err != nil {
		t.Fatalf("claude-only import failed: %v", err)
	}
	if len(res.Bundle.Artifacts) != 1 {
		t.Fatalf("artifacts = %d, want 1", len(res.Bundle.Artifacts))
	}
	a := res.Bundle.Artifacts[0]
	if _, exists := a.Overrides[model.RuntimeCodex]; exists {
		t.Fatalf("claude-only artifact must not carry a Codex override")
	}
	if len(a.Runtimes) != 1 || a.Runtimes[0] != model.RuntimeClaude {
		t.Fatalf("runtimes = %v, want [claude]", a.Runtimes)
	}
	if !a.RuntimesDeclared {
		t.Fatalf("RuntimesDeclared = false, want true — sync's bundle inheritance keys on it")
	}
}

func TestSerializeRuntimeOverridesIsDeterministic(t *testing.T) {
	a := &model.Artifact{
		Name: "demo", Type: model.TypeSkill, Description: "Demo.", Version: "1.0.0",
		Body: "Claude.\n",
		Overrides: map[model.Runtime]model.Override{
			model.RuntimeGemini: {
				Fields: map[string]any{"model": "g", "description": "Gemini."},
				Body:   "# diverged: Gemini body\nGemini.\n",
			},
			model.RuntimeCodex: {
				Fields: map[string]any{"model": "c", "description": "Codex."},
				Body:   "# diverged: Codex body\nCodex.\n",
			},
		},
	}
	first, err := serializeArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := serializeArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("runtime override serialization is nondeterministic")
	}
	if strings.Index(string(first), "  codex:") > strings.Index(string(first), "  gemini:") {
		t.Fatalf("runtime keys not sorted:\n%s", first)
	}
	if strings.Index(string(first), "    description:") > strings.Index(string(first), "    model:") {
		t.Fatalf("override fields not sorted:\n%s", first)
	}
}
