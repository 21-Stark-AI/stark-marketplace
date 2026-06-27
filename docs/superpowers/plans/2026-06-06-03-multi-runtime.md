# stark-marketplace — Slice 3: Multi-runtime (Codex + Gemini targets) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Codex and Gemini adapter targets to the engine so a canonical artifact emits correct native output on all three runtimes — per the corrected §6 capability matrix — with per-field capability fallback, the §6.1 emulation fidelity header, §6.3 cross-artifact aggregation into shared files (`AGENTS.md`, `GEMINI.md`, `config.toml`) via sorted/idempotent sentinels, and §7.7 independent per-target versioning. `codex/` and `gemini/` dist trees are **built but not committed**.

**Architecture:** Two new `adapter.Target` implementations (`internal/adapter/codex`, `internal/adapter/gemini`) build on the slice-02 canonical bundle-level adapter interface (CC-1) — each implements `Render(b *model.Bundle)`, iterating `b.Artifacts` and resolving bodies via `merge.Resolve` (which runs `fence.Strip`) internally. A shared, **versioned capability matrix** (`internal/adapter/capability`) declares per-(type,runtime) support levels; a table-driven **field map** (`internal/fieldmap`) declares per-(field,runtime) carry/map/drop+warn/error behavior. Emulated outputs are synthesized by the target from canonical fields (never authored) and prefixed with the fidelity header (`internal/adapter/emulate`). A pure **aggregator** (`internal/aggregate`) merges N per-artifact contributions destined for one shared file into a single deterministic, sentinel-wrapped, sorted, idempotent document. Validation surfaces emulated (warn) / unsupported (error) using the same matrix.

**Tech Stack:** Go 1.23 (pinned `toolchain`), `github.com/pelletier/go-toml/v2` (Codex/Gemini TOML emission — emit only; comment-aware editing is the INSTALL slice), `gopkg.in/yaml.v3` (already present), standard `testing`. TOML is marshaled from **ordered structs** (never Go maps) so key order is deterministic.

**Consistency anchor:** This plan reuses the package layout + domain types from **plan 01** (`model.Artifact`, `model.Bundle`, `model.Runtime`, `model.ArtifactType`, `model.SupportLevel`, `fence.Strip`, `merge.Resolve`, `validate.Result`) verbatim, and the **plan-02 canonical bundle-level adapter interface (CC-1)**:

```go
// package internal/adapter
type OutputFile struct { Path string; Content []byte }
type Finding    struct { Where string; Level string; Msg string } // Level: "warn"|"error"
type Target interface {
	Runtime() model.Runtime
	Version() string
	Render(b *model.Bundle) ([]OutputFile, []Finding, error) // bundle-level
}
```

Targets iterate `b.Artifacts`, calling `merge.Resolve(a, rt)` (which runs `fence.Strip`)
**internally** to obtain the resolved body — they do **not** receive a pre-stripped `body`
parameter. This plan implements the Codex and Gemini targets against this exact signature.

> **Requires plan 02 merged first.** The `adapter` package (`Target`, `OutputFile`,
> `Finding`) and `merge.Resolve` are owned and created by plan 02; this slice does not
> redeclare them.

---

## A. File / package structure

```
engine/
  internal/
    adapter/
      adapter.go                       # Target + OutputFile + Finding + merge.Resolve (from plan 02; required, not created here)
      capability/matrix.go             # versioned (type,runtime) -> SupportLevel (Task 2)
      capability/matrix_test.go
      emulate/header.go                # §6.1 fidelity header synth (Task 3)
      emulate/header_test.go
      codex/codex.go                   # Codex target (Task 6,7,8)
      codex/codex_test.go
      codex/testdata/...               # golden fixtures
      gemini/gemini.go                 # Gemini target (Task 9,10,11)
      gemini/gemini_test.go
      gemini/testdata/...              # golden fixtures
    fieldmap/fieldmap.go               # §6.2 per-field fallback table (Task 4,5)
    fieldmap/fieldmap_test.go
    aggregate/aggregate.go             # §6.3 shared-file sentinel aggregation (Task 12,13)
    aggregate/aggregate_test.go
    validate/rules_capability.go       # warn emulated / error unsupported (Task 14)
    validate/capability_test.go
```

Every step runs from the repo root unless noted. **Go commands run from `engine/`.**

Naming used verbatim by later slices (install/web): `capability.Level`, `capability.Version`, `fieldmap.Apply`, `fieldmap.Action`, `aggregate.Merge`, `aggregate.Section`, `emulate.Header`, `codex.New()`, `gemini.New()`.

---

### Task 1: Verify adapter interface + add go-toml dependency

**Files:**
- Modify: `engine/go.mod`, `engine/go.sum`

> **Requires plan 02 merged first.** The canonical adapter contract (CC-1) —
> `adapter.Target`, `adapter.OutputFile`, `adapter.Finding`, and `merge.Resolve` — is
> owned and created by plan 02. Do not redeclare them here.

- [ ] **Step 1: Verify the canonical adapter interface exists**

Confirm `engine/internal/adapter/adapter.go` exists and declares the canonical bundle-level
contract:
```go
// package internal/adapter
type OutputFile struct { Path string; Content []byte }
type Finding    struct { Where string; Level string; Msg string } // Level: "warn"|"error"
type Target interface {
	Runtime() model.Runtime
	Version() string
	Render(b *model.Bundle) ([]OutputFile, []Finding, error)
}
```
Run: `cd engine && go build ./internal/adapter/ && cd ..` — must compile. If this fails,
plan 02 is not merged; stop and merge it first (this slice has a hard dependency on it).

- [ ] **Step 2: Add the TOML dependency**

Run:
```bash
cd engine && go get github.com/pelletier/go-toml/v2@latest && go build ./... && cd ..
```
Expected: builds; `go.sum` updated with `pelletier/go-toml/v2`.

- [ ] **Step 3: Commit**

```bash
git add engine/go.mod engine/go.sum
git commit -m "feat(engine): go-toml/v2 dep for multi-runtime targets"
```

---

### Task 2: Versioned capability matrix

**Files:**
- Create: `engine/internal/adapter/capability/matrix.go`
- Test: `engine/internal/adapter/capability/matrix_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/adapter/capability/matrix_test.go`:
```go
package capability

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestLevelsPerCorrectedMatrix(t *testing.T) {
	cases := []struct {
		typ  model.ArtifactType
		rt   model.Runtime
		want model.SupportLevel
	}{
		// Codex: skills are NATIVE; prompt/command map to native skill; agent emulated; mcp native.
		{model.TypeSkill, model.RuntimeCodex, model.SupportNative},
		{model.TypePrompt, model.RuntimeCodex, model.SupportNative},
		{model.TypeCommand, model.RuntimeCodex, model.SupportNative},
		{model.TypeAgent, model.RuntimeCodex, model.SupportEmulated},
		{model.TypeMCP, model.RuntimeCodex, model.SupportNative},
		// Gemini: prompt/command native (.toml); skill/agent emulated (GEMINI.md); mcp native.
		{model.TypePrompt, model.RuntimeGemini, model.SupportNative},
		{model.TypeCommand, model.RuntimeGemini, model.SupportNative},
		{model.TypeSkill, model.RuntimeGemini, model.SupportEmulated},
		{model.TypeAgent, model.RuntimeGemini, model.SupportEmulated},
		{model.TypeMCP, model.RuntimeGemini, model.SupportNative},
		// Claude: all native (anchor for completeness).
		{model.TypeSkill, model.RuntimeClaude, model.SupportNative},
		{model.TypeAgent, model.RuntimeClaude, model.SupportNative},
	}
	for _, c := range cases {
		if got := Level(c.typ, c.rt); got != c.want {
			t.Errorf("Level(%s,%s) = %q, want %q", c.typ, c.rt, got, c.want)
		}
	}
}

func TestUnknownPairIsUnsupported(t *testing.T) {
	if got := Level("bogus", model.RuntimeCodex); got != model.SupportUnsupported {
		t.Fatalf("unknown type should be unsupported, got %q", got)
	}
}

func TestVersionIsStable(t *testing.T) {
	if Version == "" {
		t.Fatal("capability matrix must declare a version")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/adapter/capability/ -v`
Expected: FAIL — undefined `Level`, `Version`.

- [ ] **Step 3: Implement the matrix**

`engine/internal/adapter/capability/matrix.go`:
```go
// Package capability is the versioned source of truth for per-(type,runtime)
// support levels (spec §6). It is plain data — surfaced in the index for
// native/emulated/unsupported badges and consumed by validation (§7.4).
package capability

import "github.com/21-Stark-AI/stark-marketplace/engine/internal/model"

// Version bumps whenever any cell changes; it is independent of adapter target
// versions (§7.7) and lets the index communicate matrix revisions.
const Version = "1"

type key struct {
	t  model.ArtifactType
	rt model.Runtime
}

// matrix encodes the corrected §6 capability matrix (red-team Part B).
var matrix = map[key]model.SupportLevel{
	// ── Claude Code: everything native ──
	{model.TypeSkill, model.RuntimeClaude}:   model.SupportNative,
	{model.TypePrompt, model.RuntimeClaude}:  model.SupportNative,
	{model.TypeCommand, model.RuntimeClaude}: model.SupportNative,
	{model.TypeAgent, model.RuntimeClaude}:   model.SupportNative,
	{model.TypeMCP, model.RuntimeClaude}:     model.SupportNative,

	// ── Codex (OpenAI): native Skills at .agents/skills/<name>/SKILL.md ──
	// prompts deprecated → command/prompt map to a Codex skill (still native shape).
	{model.TypeSkill, model.RuntimeCodex}:   model.SupportNative,
	{model.TypePrompt, model.RuntimeCodex}:  model.SupportNative,
	{model.TypeCommand, model.RuntimeCodex}: model.SupportNative,
	{model.TypeAgent, model.RuntimeCodex}:   model.SupportEmulated, // no subagent primitive
	{model.TypeMCP, model.RuntimeCodex}:     model.SupportNative,   // ~/.codex/config.toml [mcp_servers.<name>]

	// ── Gemini CLI ──
	{model.TypePrompt, model.RuntimeGemini}:  model.SupportNative, // .gemini/commands/<name>.toml
	{model.TypeCommand, model.RuntimeGemini}: model.SupportNative,
	{model.TypeSkill, model.RuntimeGemini}:   model.SupportEmulated, // GEMINI.md sentinel block
	{model.TypeAgent, model.RuntimeGemini}:   model.SupportEmulated, // GEMINI.md role block
	{model.TypeMCP, model.RuntimeGemini}:     model.SupportNative,   // settings.json mcpServers.<name>
}

// Level returns the support level for a (type, runtime) pair. Unknown pairs are
// treated as unsupported (fail-closed).
func Level(t model.ArtifactType, rt model.Runtime) model.SupportLevel {
	if lvl, ok := matrix[key{t, rt}]; ok {
		return lvl
	}
	return model.SupportUnsupported
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/adapter/capability/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/adapter/capability/
git commit -m "feat(adapter): versioned capability matrix (corrected §6, fail-closed)"
```

---

### Task 3: Emulation fidelity header

**Files:**
- Create: `engine/internal/adapter/emulate/header.go`
- Test: `engine/internal/adapter/emulate/header_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/adapter/emulate/header_test.go`:
```go
package emulate

import (
	"strings"
	"testing"
)

func TestHeaderContainsProvenanceAndWarning(t *testing.T) {
	h := Header("stark-review", "red-team", "<!-- ", " -->")
	if !strings.Contains(h, "EMULATED from stark-review/red-team") {
		t.Fatalf("missing provenance: %q", h)
	}
	if !strings.Contains(h, "may not auto-activate") || !strings.Contains(h, "verify") {
		t.Fatalf("missing fidelity warning: %q", h)
	}
	// Must use the supplied comment delimiters so it is inert in the target file.
	if !strings.HasPrefix(h, "<!-- ") {
		t.Fatalf("header must be wrapped in comment open delim: %q", h)
	}
	if !strings.HasSuffix(h, "\n") {
		t.Fatal("header must end with a newline so it precedes content cleanly")
	}
}

func TestHeaderDeterministic(t *testing.T) {
	a := Header("b", "n", "# ", "")
	b := Header("b", "n", "# ", "")
	if a != b {
		t.Fatal("header must be a pure function of its inputs (no clock/random)")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/adapter/emulate/ -v`
Expected: FAIL — undefined `Header`.

- [ ] **Step 3: Implement**

`engine/internal/adapter/emulate/header.go`:
```go
// Package emulate synthesizes the shape + fidelity header for emulated outputs
// (spec §6.1). Emulation shape is adapter-owned and NEVER authored via overrides.
package emulate

import "fmt"

// Header returns the generated fidelity header for an emulated artifact, wrapped
// in the target file's comment delimiters (e.g. "<!-- "/" -->" for markdown,
// "# "/"" for TOML). It is a pure function — no clock, no randomness — so output
// stays byte-stable (spec §7.6).
func Header(bundle, artifact, open, close string) string {
	const tmpl = "EMULATED from %s/%s — derived shape; may not auto-activate on this runtime; verify."
	return fmt.Sprintf("%s%s%s\n", open, fmt.Sprintf(tmpl, bundle, artifact), close)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/adapter/emulate/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/adapter/emulate/
git commit -m "feat(adapter): emulation fidelity header (§6.1, pure)"
```

---

### Task 4: Per-field capability fallback — table + Action type

**Files:**
- Create: `engine/internal/fieldmap/fieldmap.go`
- Test: `engine/internal/fieldmap/fieldmap_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/fieldmap/fieldmap_test.go`:
```go
package fieldmap

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestActionsMatchSpec62(t *testing.T) {
	// spec §6.2 contract table.
	cases := []struct {
		field string
		rt    model.Runtime
		want  Action
	}{
		{"model", model.RuntimeClaude, ActionCarry},
		{"model", model.RuntimeCodex, ActionMap},
		{"model", model.RuntimeGemini, ActionDrop},
		{"argument-hint", model.RuntimeCodex, ActionDerive},
		{"argument-hint", model.RuntimeGemini, ActionDerive},
		{"disable-model-invocation", model.RuntimeCodex, ActionDrop},
		{"disable-model-invocation", model.RuntimeGemini, ActionDrop},
		{"allowed-tools", model.RuntimeCodex, ActionBestEffort},
		{"allowed-tools", model.RuntimeGemini, ActionDrop},
	}
	for _, c := range cases {
		if got := actionFor(c.field, c.rt); got != c.want {
			t.Errorf("actionFor(%s,%s) = %q, want %q", c.field, c.rt, got, c.want)
		}
	}
}

func TestUnknownFieldCarries(t *testing.T) {
	// fields with no entry default to carry (Claude-native passthrough).
	if got := actionFor("category", model.RuntimeClaude); got != ActionCarry {
		t.Fatalf("default should be carry, got %q", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/fieldmap/ -v`
Expected: FAIL — undefined `Action`, `ActionCarry`, `actionFor`.

- [ ] **Step 3: Implement the table**

`engine/internal/fieldmap/fieldmap.go`:
```go
// Package fieldmap is the table-driven per-field capability fallback (spec §6.2).
// It tells each adapter target how to treat a canonical field on a given runtime:
// carry it natively, translate its value, drop it (counting a warning), derive it
// into prose, or block.
package fieldmap

import "github.com/21-Stark-AI/stark-marketplace/engine/internal/model"

type Action string

const (
	ActionCarry      Action = "carry"       // emit as a native field
	ActionMap        Action = "map"         // translate the value
	ActionDrop       Action = "drop+warn"   // omit; count a warning
	ActionDerive     Action = "derive"      // render into usage/prose text
	ActionBestEffort Action = "best-effort" // emit as best-effort metadata
	ActionError      Action = "error"       // block the build
)

type key struct {
	field string
	rt    model.Runtime
}

// table is the §6.2 contract for the common canonical fields. Anything not listed
// defaults to carry.
var table = map[key]Action{
	// model
	{"model", model.RuntimeClaude}: ActionCarry, // skill: only with context:fork (target enforces)
	{"model", model.RuntimeCodex}:  ActionMap,
	{"model", model.RuntimeGemini}: ActionDrop,
	// argument-hint
	{"argument-hint", model.RuntimeClaude}: ActionCarry,
	{"argument-hint", model.RuntimeCodex}:  ActionDerive,
	{"argument-hint", model.RuntimeGemini}: ActionDerive,
	// disable-model-invocation
	{"disable-model-invocation", model.RuntimeClaude}: ActionCarry,
	{"disable-model-invocation", model.RuntimeCodex}:  ActionDrop,
	{"disable-model-invocation", model.RuntimeGemini}: ActionDrop,
	// allowed-tools / tools
	{"allowed-tools", model.RuntimeClaude}: ActionCarry,
	{"allowed-tools", model.RuntimeCodex}:  ActionBestEffort,
	{"allowed-tools", model.RuntimeGemini}: ActionDrop,
}

func actionFor(field string, rt model.Runtime) Action {
	if a, ok := table[key{field, rt}]; ok {
		return a
	}
	return ActionCarry
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/fieldmap/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/fieldmap/
git commit -m "feat(fieldmap): §6.2 per-field capability fallback table"
```

---

### Task 5: Field map `Apply` — resolve fields + collect warnings

**Files:**
- Modify: `engine/internal/fieldmap/fieldmap.go`
- Test: `engine/internal/fieldmap/apply_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/fieldmap/apply_test.go`:
```go
package fieldmap

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestApplyDropsAndWarns(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand,
		Model: "opus", ArgumentHint: "[PR]", DisableModelInvocation: true,
		AllowedTools: []string{"Bash"},
	}
	res := Apply(a, model.RuntimeGemini, codexModelMapNoop)
	// Gemini drops model, disable-model-invocation, allowed-tools → 3 warnings.
	if len(res.Dropped) != 3 {
		t.Fatalf("want 3 dropped fields, got %v", res.Dropped)
	}
	if _, ok := res.Carried["model"]; ok {
		t.Fatal("model should not be carried on gemini")
	}
	// argument-hint is derived, not carried, not dropped.
	if res.Derived["argument-hint"] != "[PR]" {
		t.Fatalf("argument-hint should be derived, got %v", res.Derived)
	}
}

func TestApplyMapsCodexModel(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, Model: "opus"}
	mapper := func(v string) (string, bool) {
		if v == "opus" {
			return "gpt-5-codex", true
		}
		return "", false
	}
	res := Apply(a, model.RuntimeCodex, mapper)
	if res.Carried["model"] != "gpt-5-codex" {
		t.Fatalf("codex model should map opus→gpt-5-codex, got %q", res.Carried["model"])
	}
}

func TestApplyMapMissTargetDrops(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill, Model: "weird-model"}
	mapper := func(string) (string, bool) { return "", false }
	res := Apply(a, model.RuntimeCodex, mapper)
	if _, ok := res.Carried["model"]; ok {
		t.Fatal("unmappable model must drop")
	}
	if len(res.Dropped) != 1 || res.Dropped[0] != "model" {
		t.Fatalf("want model dropped, got %v", res.Dropped)
	}
}

func codexModelMapNoop(v string) (string, bool) { return v, true }
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/fieldmap/ -run TestApply -v`
Expected: FAIL — undefined `Apply`, `Result`.

- [ ] **Step 3: Implement `Apply`**

Append to `engine/internal/fieldmap/fieldmap.go`:
```go
// ModelMapper translates a canonical model id into a runtime-specific id. ok=false
// means "no mapping" → the field drops with a warning.
type ModelMapper func(canonical string) (string, bool)

// Result is the outcome of applying the field map to one artifact for one runtime.
type Result struct {
	Carried map[string]string // field -> native value to emit
	Derived map[string]string // field -> value to render into usage/prose
	Dropped []string          // fields omitted; caller counts these as warnings
}

// Apply walks the common canonical fields present on a, resolves each via the
// §6.2 table, and partitions them into carried / derived / dropped. modelMap is
// consulted only for ActionMap on the `model` field.
func Apply(a *model.Artifact, rt model.Runtime, modelMap ModelMapper) Result {
	res := Result{Carried: map[string]string{}, Derived: map[string]string{}}

	type field struct {
		name    string
		present bool
		value   string
	}
	fields := []field{
		{"model", a.Model != "", a.Model},
		{"argument-hint", a.ArgumentHint != "", a.ArgumentHint},
		{"disable-model-invocation", a.DisableModelInvocation, boolStr(a.DisableModelInvocation)},
		{"allowed-tools", len(a.AllowedTools) > 0, joinTools(a.AllowedTools)},
	}

	for _, f := range fields {
		if !f.present {
			continue
		}
		switch actionFor(f.name, rt) {
		case ActionCarry, ActionBestEffort:
			res.Carried[f.name] = f.value
		case ActionDerive:
			res.Derived[f.name] = f.value
		case ActionMap:
			if f.name == "model" && modelMap != nil {
				if mapped, ok := modelMap(f.value); ok {
					res.Carried[f.name] = mapped
					continue
				}
			}
			res.Dropped = append(res.Dropped, f.name)
		case ActionDrop, ActionError:
			res.Dropped = append(res.Dropped, f.name)
		}
	}
	return res
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func joinTools(t []string) string {
	out := ""
	for i, s := range t {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/fieldmap/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/fieldmap/
git commit -m "feat(fieldmap): Apply resolves fields into carried/derived/dropped"
```

---

### Task 6: Codex target — skill native output (skill/prompt/command)

**Files:**
- Create: `engine/internal/adapter/codex/codex.go`
- Test: `engine/internal/adapter/codex/codex_test.go`

- [ ] **Step 1: Write the failing test (native skill SKILL.md)**

The target implements the canonical bundle-level `Render(b *model.Bundle)` (CC-1): it
iterates `b.Artifacts`, calls `merge.Resolve(a, model.RuntimeCodex)` internally to get each
resolved body (no `body` param), and returns `([]adapter.OutputFile, []adapter.Finding, error)`.

`engine/internal/adapter/codex/codex_test.go`:
```go
package codex

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func findFile(files []adapter.OutputFile, suffix string) (string, bool) {
	for _, f := range files {
		if strings.HasSuffix(f.Path, suffix) {
			return string(f.Content), true
		}
	}
	return "", false
}

// bundleWith wraps one artifact in a single-artifact bundle for target tests.
// merge.Resolve reads a.Body + a.Runtimes to produce the resolved, fence-stripped body.
func bundleWith(a *model.Artifact) *model.Bundle {
	return &model.Bundle{Name: a.Bundle, Artifacts: []*model.Artifact{a}}
}

func TestCodexEmitsNativeSkill(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "Single-agent PR review.", Version: "0.7.0",
		Body:     "Do the review.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, ".agents/skills/stark-review/SKILL.md")
	if !ok {
		t.Fatalf("expected native Codex skill path; got %v", files)
	}
	// name + description required in frontmatter; no fidelity header (native).
	if !contains(body, "name: stark-review") || !contains(body, "description: Single-agent PR review.") {
		t.Fatalf("missing required frontmatter: %q", body)
	}
	if contains(body, "EMULATED from") {
		t.Fatal("native skill must NOT carry an emulation header")
	}
	if !contains(body, "Do the review.") {
		t.Fatalf("body missing: %q", body)
	}
}

func TestCodexMapsCommandToSkillWithUsage(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand, Bundle: "stark-review",
		Description: "PR review command.", Version: "0.7.0",
		ArgumentHint: "[PR_NUMBER]", Body: "Review the PR.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, ".agents/skills/review/SKILL.md")
	if !ok {
		t.Fatalf("command must map to a Codex skill; got %v", files)
	}
	// argument-hint is derived into usage prose (§6.2).
	if !contains(body, "Usage:") || !contains(body, "[PR_NUMBER]") {
		t.Fatalf("derived usage missing: %q", body)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/adapter/codex/ -v`
Expected: FAIL — undefined `New`.

- [ ] **Step 3: Implement the Codex target (skill + command/prompt → skill)**

`engine/internal/adapter/codex/codex.go`:
```go
// Package codex is the Codex (OpenAI) adapter target (spec §6, corrected matrix).
// Codex has NATIVE Skills at .agents/skills/<name>/SKILL.md (name+description
// required). Prompts are deprecated, so prompt/command map to a Codex skill.
// agent → emulated Codex skill. mcp → ~/.codex/config.toml [mcp_servers.<name>].
// Render is the canonical bundle-level entry point (CC-1): it iterates the bundle's
// artifacts, resolves each body via merge.Resolve (which runs fence.Strip) internally,
// and emits per-runtime output.
package codex

import (
	"fmt"
	"sort"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/emulate"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/fieldmap"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/merge"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// version is the independently-versioned target identity (spec §7.7).
const version = "codex@1"

type Target struct{}

func New() *Target { return &Target{} }

func (t *Target) Runtime() model.Runtime { return model.RuntimeCodex }
func (t *Target) Version() string        { return version }

// modelMap translates canonical model ids → Codex model ids (§6.2 ActionMap).
func modelMap(canonical string) (string, bool) {
	switch canonical {
	case "opus", "sonnet":
		return "gpt-5-codex", true
	case "haiku":
		return "gpt-5-mini", true
	default:
		return "", false
	}
}

// Render emits Codex output for every artifact in the bundle that targets Codex.
// Per CC-1 it owns body resolution: merge.Resolve(a, RuntimeCodex) runs fence.Strip
// internally — the target never receives a pre-stripped body.
func (t *Target) Render(b *model.Bundle) ([]adapter.OutputFile, []adapter.Finding, error) {
	var files []adapter.OutputFile
	var findings []adapter.Finding
	for _, a := range b.Artifacts {
		if !targetsRuntime(a, model.RuntimeCodex) {
			continue
		}
		body, err := merge.Resolve(a, model.RuntimeCodex)
		if err != nil {
			return nil, nil, fmt.Errorf("codex: resolve %s/%s: %w", b.Name, a.Name, err)
		}
		out, err := t.emitArtifact(a, body)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, out...)
	}
	return files, findings, nil
}

func targetsRuntime(a *model.Artifact, rt model.Runtime) bool {
	for _, r := range a.Runtimes {
		if r == rt {
			return true
		}
	}
	return false
}

func (t *Target) emitArtifact(a *model.Artifact, body string) ([]adapter.OutputFile, error) {
	switch a.Type {
	case model.TypeSkill, model.TypePrompt, model.TypeCommand:
		return t.emitSkill(a, body, false), nil
	case model.TypeAgent:
		return t.emitSkill(a, body, true), nil // emulated
	case model.TypeMCP:
		return t.emitMCP(a)
	default:
		return nil, fmt.Errorf("codex: unsupported artifact type %q", a.Type)
	}
}

// emitSkill writes .agents/skills/<name>/SKILL.md. emulated=true prepends the
// §6.1 fidelity header (agents have no Codex primitive).
func (t *Target) emitSkill(a *model.Artifact, body string, emulated bool) []adapter.OutputFile {
	res := fieldmap.Apply(a, model.RuntimeCodex, modelMap)

	var fm strings.Builder
	fm.WriteString("---\n")
	// name + description are REQUIRED by Codex skills.
	fm.WriteString("name: " + a.Name + "\n")
	fm.WriteString("description: " + a.Description + "\n")
	// carried fields, sorted by key for determinism (§7.6).
	for _, k := range sortedKeys(res.Carried) {
		fm.WriteString(k + ": " + res.Carried[k] + "\n")
	}
	fm.WriteString("---\n")

	var b strings.Builder
	if emulated {
		b.WriteString(emulate.Header(a.Bundle, a.Name, "<!-- ", " -->"))
	}
	// derived fields render as usage prose (§6.2: argument-hint → usage note).
	if hint, ok := res.Derived["argument-hint"]; ok {
		b.WriteString("Usage: " + a.Name + " " + hint + "\n\n")
	}
	b.WriteString(body)

	return []adapter.OutputFile{{
		Path:    ".agents/skills/" + a.Name + "/SKILL.md",
		Content: []byte(fm.String() + b.String()),
	}}
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
```

> `emitMCP` is added in Task 7; for now add a stub so the package compiles:
```go
func (t *Target) emitMCP(a *model.Artifact) ([]adapter.OutputFile, error) {
	return nil, fmt.Errorf("codex: mcp emit not yet implemented")
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/adapter/codex/ -run 'TestCodexEmitsNativeSkill|TestCodexMapsCommandToSkillWithUsage' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/adapter/codex/
git commit -m "feat(codex): native Skills emit (skill/prompt/command → SKILL.md) + agent emulation"
```

---

### Task 7: Codex target — MCP into config.toml (deterministic TOML)

**Files:**
- Modify: `engine/internal/adapter/codex/codex.go` (replace `emitMCP` stub)
- Test: `engine/internal/adapter/codex/mcp_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/adapter/codex/mcp_test.go`:
```go
package codex

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestCodexEmitsMCPConfigToml(t *testing.T) {
	a := &model.Artifact{
		Name: "bigquery", Type: model.TypeMCP, Bundle: "stark-data",
		Description: "BQ MCP.", Version: "1.2.0",
		Runtimes: []model.Runtime{model.RuntimeCodex},
		MCP: &model.MCPConfig{
			Transport: "stdio", Command: "stark-bq-mcp",
			Args: []string{"--project", "${BQ_PROJECT}"},
			Env:  map[string]model.SecretRef{"BQ_PROJECT": {SecretRef: "bq-project-id"}},
		},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := findFile(files, "config.toml")
	if !ok {
		t.Fatalf("expected config.toml; got %v", files)
	}
	// Codex MCP key is [mcp_servers.<name>] (spec §6, open-q #1 pinned here).
	for _, want := range []string{
		"[mcp_servers.bigquery]",
		`command = "stark-bq-mcp"`,
		`args = ["--project", "${BQ_PROJECT}"]`,
		`[mcp_servers.bigquery.env]`,
		// secretRef is referenced by name, never the value (spec §4.4).
		`BQ_PROJECT = "${BQ_PROJECT}"`,
	} {
		if !contains(body, want) {
			t.Fatalf("config.toml missing %q in:\n%s", want, body)
		}
	}
}
```

> The env value renders as `${<KEY>}` (a placeholder the developer's shell/secret
> tooling resolves), **never** the secret's catalog key or value — keeping the
> catalog secret-free (§4.4). The secretRef name (`bq-project-id`) is recorded in
> the install manifest, not the emitted file.

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/adapter/codex/ -run TestCodexEmitsMCP -v`
Expected: FAIL — `emitMCP not yet implemented`.

- [ ] **Step 3: Implement `emitMCP` with ordered TOML structs**

Replace the `emitMCP` stub in `codex.go` and add imports (`"github.com/pelletier/go-toml/v2"`):
```go
// codexMCPDoc is an ordered struct (NOT a map) so go-toml emits deterministic
// key order (§7.6). One server per emitted fragment; install merges by key (§9.2).
type codexMCPDoc struct {
	MCPServers map[string]codexMCPServer `toml:"mcp_servers"`
}

type codexMCPServer struct {
	Command string            `toml:"command,omitempty"`
	Args    []string          `toml:"args,omitempty"`
	URL     string            `toml:"url,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

func (t *Target) emitMCP(a *model.Artifact) ([]adapter.OutputFile, error) {
	if a.MCP == nil {
		return nil, fmt.Errorf("codex: mcp artifact %q has no mcp config", a.Name)
	}
	srv := codexMCPServer{
		Command: a.MCP.Command,
		Args:    a.MCP.Args,
		URL:     a.MCP.URL,
	}
	if len(a.MCP.Env) > 0 {
		srv.Env = map[string]string{}
		// secretRef → ${KEY} placeholder; never the secret value (§4.4).
		for k := range a.MCP.Env {
			srv.Env[k] = "${" + k + "}"
		}
	}
	doc := codexMCPDoc{MCPServers: map[string]codexMCPServer{a.Name: srv}}
	out, err := toml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("codex: marshal mcp toml: %w", err)
	}
	return []adapter.OutputFile{{Path: "config.toml", Content: out}}, nil
}
```

> go-toml/v2 sorts `map` keys lexically on marshal and the single-element
> `mcp_servers` map plus the alphabetized `env` map give stable output. The
> `args` slice preserves source order (intentional — args are positional).

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/adapter/codex/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/adapter/codex/
git commit -m "feat(codex): mcp → config.toml [mcp_servers.<name>] (deterministic, secret-free)"
```

---

### Task 8: Codex golden-file test (byte-exact)

**Files:**
- Create: `engine/internal/adapter/codex/testdata/skill.golden`
- Create: `engine/internal/adapter/codex/testdata/mcp.golden`
- Test: `engine/internal/adapter/codex/golden_test.go`

- [ ] **Step 1: Write the golden test (canonical fixture → byte-exact output)**

`engine/internal/adapter/codex/golden_test.go`:
```go
package codex

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

func goldenSkill() *model.Artifact {
	return &model.Artifact{
		Name: "stark-review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "Single-agent PR review.", Version: "0.7.0",
		Model: "opus", // maps → gpt-5-codex
		Body:  "Do the review.\n",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
}

func goldenMCP() *model.Artifact {
	return &model.Artifact{
		Name: "bigquery", Type: model.TypeMCP, Bundle: "stark-data",
		Description: "BQ MCP.", Version: "1.2.0",
		Runtimes: []model.Runtime{model.RuntimeCodex},
		MCP: &model.MCPConfig{
			Transport: "stdio", Command: "stark-bq-mcp",
			Args: []string{"--project", "${BQ_PROJECT}"},
			Env:  map[string]model.SecretRef{"BQ_PROJECT": {SecretRef: "bq-project-id"}},
		},
	}
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run -update)", name, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenCodexSkill(t *testing.T) {
	files, _, _ := New().Render(bundleWith(goldenSkill()))
	assertGolden(t, "skill.golden", files[0].Content)
}

func TestGoldenCodexMCP(t *testing.T) {
	files, _, _ := New().Render(bundleWith(goldenMCP()))
	assertGolden(t, "mcp.golden", files[0].Content)
}
```

- [ ] **Step 2: Generate the goldens (first run with -update), then verify locked**

Run:
```bash
cd engine && go test ./internal/adapter/codex/ -run TestGolden -update && go test ./internal/adapter/codex/ -run TestGolden -v && cd ..
```
Expected: first call writes `testdata/skill.golden` + `testdata/mcp.golden`; second call PASSES against them. Inspect the goldens: `skill.golden` must contain `name: stark-review`, `description: …`, `model: gpt-5-codex` (opus mapped), no `EMULATED`; `mcp.golden` must contain `[mcp_servers.bigquery]`.

- [ ] **Step 3: Commit**

```bash
git add engine/internal/adapter/codex/testdata/ engine/internal/adapter/codex/golden_test.go
git commit -m "test(codex): byte-exact golden fixtures (native skill + mcp)"
```

---

### Task 9: Gemini target — commands/prompts → .gemini/commands/<name>.toml

**Files:**
- Create: `engine/internal/adapter/gemini/gemini.go`
- Test: `engine/internal/adapter/gemini/gemini_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/adapter/gemini/gemini_test.go`:
```go
package gemini

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func find(files []adapter.OutputFile, suffix string) (string, bool) {
	for _, f := range files {
		if strings.HasSuffix(f.Path, suffix) {
			return string(f.Content), true
		}
	}
	return "", false
}

// bundleWith wraps one artifact in a single-artifact bundle for target tests.
// merge.Resolve reads a.Body + a.Runtimes to produce the resolved, fence-stripped body.
func bundleWith(a *model.Artifact) *model.Bundle {
	return &model.Bundle{Name: a.Bundle, Artifacts: []*model.Artifact{a}}
}

func TestGeminiEmitsCommandToml(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand, Bundle: "stark-review",
		Description: "PR review command.", Version: "0.7.0",
		ArgumentHint: "[PR_NUMBER]", Body: "Review the PR for {{args}}.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := find(files, ".gemini/commands/review.toml")
	if !ok {
		t.Fatalf("expected .gemini/commands/review.toml; got %v", files)
	}
	// Gemini command TOML has ONLY prompt + description (spec §6).
	if !strings.Contains(body, `description = "PR review command."`) {
		t.Fatalf("missing description: %q", body)
	}
	if !strings.Contains(body, "prompt =") || !strings.Contains(body, "{{args}}") {
		t.Fatalf("missing prompt/{{args}}: %q", body)
	}
	// argument-hint is derived into the prompt usage note (no native field).
	if !strings.Contains(body, "[PR_NUMBER]") {
		t.Fatalf("derived arg-hint missing: %q", body)
	}
	// No model field on Gemini commands.
	if strings.Contains(body, "model =") {
		t.Fatal("gemini command toml must not carry model")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/adapter/gemini/ -v`
Expected: FAIL — undefined `New`.

- [ ] **Step 3: Implement the Gemini target (command/prompt path)**

`engine/internal/adapter/gemini/gemini.go`:
```go
// Package gemini is the Gemini CLI adapter target (spec §6).
// command/prompt → .gemini/commands/<name>.toml (prompt + description ONLY; args
// via {{args}}). skill/agent → emulated GEMINI.md sentinel blocks. mcp →
// settings.json mcpServers.<name>.
//
// OPEN QUESTION (spec §15.2): Gemini Extensions may be a more faithful target for
// skill/agent emulation (installable/uninstallable cleanly). This slice emits
// GEMINI.md sentinel blocks; an Extensions target can be added as gemini@2 without
// disturbing the command/mcp paths. Do not block this slice on it.
package gemini

import (
	"fmt"
	"strings"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/emulate"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/fieldmap"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/merge"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
	"github.com/pelletier/go-toml/v2"
)

const version = "gemini@1"

type Target struct{}

func New() *Target { return &Target{} }

func (t *Target) Runtime() model.Runtime { return model.RuntimeGemini }
func (t *Target) Version() string        { return version }

// Render emits Gemini output for every artifact in the bundle that targets Gemini.
// Per CC-1 it owns body resolution: merge.Resolve(a, RuntimeGemini) runs fence.Strip
// internally — the target never receives a pre-stripped body.
func (t *Target) Render(b *model.Bundle) ([]adapter.OutputFile, []adapter.Finding, error) {
	var files []adapter.OutputFile
	var findings []adapter.Finding
	for _, a := range b.Artifacts {
		if !targetsRuntime(a, model.RuntimeGemini) {
			continue
		}
		body, err := merge.Resolve(a, model.RuntimeGemini)
		if err != nil {
			return nil, nil, fmt.Errorf("gemini: resolve %s/%s: %w", b.Name, a.Name, err)
		}
		out, err := t.emitArtifact(a, body)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, out...)
	}
	return files, findings, nil
}

func targetsRuntime(a *model.Artifact, rt model.Runtime) bool {
	for _, r := range a.Runtimes {
		if r == rt {
			return true
		}
	}
	return false
}

func (t *Target) emitArtifact(a *model.Artifact, body string) ([]adapter.OutputFile, error) {
	switch a.Type {
	case model.TypeCommand, model.TypePrompt:
		return t.emitCommand(a, body)
	case model.TypeSkill, model.TypeAgent:
		return t.emitEmulated(a, body), nil
	case model.TypeMCP:
		return t.emitMCP(a)
	default:
		return nil, fmt.Errorf("gemini: unsupported artifact type %q", a.Type)
	}
}

// geminiCmd is an ordered struct: only prompt + description (§6). go-toml emits
// these struct fields in declaration order → deterministic.
type geminiCmd struct {
	Description string `toml:"description"`
	Prompt      string `toml:"prompt"`
}

func (t *Target) emitCommand(a *model.Artifact, body string) ([]adapter.OutputFile, error) {
	res := fieldmap.Apply(a, model.RuntimeGemini, nil)
	prompt := body
	if hint, ok := res.Derived["argument-hint"]; ok {
		prompt = "Usage: /" + a.Name + " " + hint + "\n\n" + body
	}
	doc := geminiCmd{Description: a.Description, Prompt: prompt}
	out, err := toml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal command toml: %w", err)
	}
	return []adapter.OutputFile{{
		Path:    ".gemini/commands/" + a.Name + ".toml",
		Content: out,
	}}, nil
}
```

> Stubs so the package compiles (filled in Tasks 10–11):
```go
func (t *Target) emitEmulated(a *model.Artifact, body string) []adapter.OutputFile {
	_ = emulate.Header // referenced in Task 10
	return nil
}
func (t *Target) emitMCP(a *model.Artifact) ([]adapter.OutputFile, error) {
	_ = strings.TrimSpace
	return nil, fmt.Errorf("gemini: mcp emit not yet implemented")
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/adapter/gemini/ -run TestGeminiEmitsCommand -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/adapter/gemini/
git commit -m "feat(gemini): command/prompt → .gemini/commands/<name>.toml (prompt+description only)"
```

---

### Task 10: Gemini target — skill/agent emulation into GEMINI.md sentinel block

**Files:**
- Modify: `engine/internal/adapter/gemini/gemini.go` (replace `emitEmulated` stub)
- Test: `engine/internal/adapter/gemini/emulate_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/adapter/gemini/emulate_test.go`:
```go
package gemini

import (
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestGeminiEmulatesSkillIntoGeminiMd(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "PR review.", Version: "0.7.0",
		Body:     "Review carefully.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := find(files, "GEMINI.md")
	if !ok {
		t.Fatalf("expected GEMINI.md; got %v", files)
	}
	// emulated → fidelity header present.
	if !strings.Contains(body, "EMULATED from stark-review/stark-review") {
		t.Fatalf("missing fidelity header: %q", body)
	}
	// wrapped in stark sentinels so install can merge by sentinel (§6.3/§9.2).
	if !strings.Contains(body, "<!-- stark:begin stark-review/stark-review@") {
		t.Fatalf("missing begin sentinel: %q", body)
	}
	if !strings.Contains(body, "<!-- stark:end stark-review/stark-review -->") {
		t.Fatalf("missing end sentinel: %q", body)
	}
	if !strings.Contains(body, "Review carefully.") {
		t.Fatalf("body missing: %q", body)
	}
}

func TestGeminiAgentEmulationIsRoleBlock(t *testing.T) {
	a := &model.Artifact{
		Name: "red-team", Type: model.TypeAgent, Bundle: "stark-review",
		Description: "Adversarial reviewer.", Version: "0.7.0",
		Body:     "Attack the design.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, _ := New().Render(bundleWith(a))
	body, _ := find(files, "GEMINI.md")
	if !strings.Contains(body, "Role: red-team") {
		t.Fatalf("agent emulation should render a role block: %q", body)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/adapter/gemini/ -run 'TestGeminiEmulates|TestGeminiAgent' -v`
Expected: FAIL (`emitEmulated` returns nil).

- [ ] **Step 3: Implement `emitEmulated` + sentinel helper**

Replace the `emitEmulated` stub in `gemini.go` and add a digest import (`"crypto/sha256"`, `"encoding/hex"`):
```go
// sectionDigest is a short content digest used in the begin sentinel so install
// can detect drift. Pure function of the rendered inner content.
func sectionDigest(inner string) string {
	sum := sha256.Sum256([]byte(inner))
	return hex.EncodeToString(sum[:])[:12]
}

func (t *Target) emitEmulated(a *model.Artifact, body string) []adapter.OutputFile {
	var inner strings.Builder
	inner.WriteString(emulate.Header(a.Bundle, a.Name, "<!-- ", " -->"))
	switch a.Type {
	case model.TypeAgent:
		inner.WriteString("## Role: " + a.Name + "\n")
		inner.WriteString(a.Description + "\n\n")
	default: // skill
		inner.WriteString("## Skill: " + a.Name + "\n")
		inner.WriteString(a.Description + "\n\n")
	}
	inner.WriteString(body)

	id := a.Bundle + "/" + a.Name
	innerStr := inner.String()
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- stark:begin %s@%s -->\n", id, sectionDigest(innerStr))
	b.WriteString(innerStr)
	if !strings.HasSuffix(innerStr, "\n") {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "<!-- stark:end %s -->\n", id)

	return []adapter.OutputFile{{Path: "GEMINI.md", Content: []byte(b.String())}}
}
```

Remove the now-unused `_ = emulate.Header` line.

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/adapter/gemini/ -v`
Expected: PASS (mcp test still pending — it is in Task 11; the mcp stub error only triggers on mcp artifacts, not these).

- [ ] **Step 5: Commit**

```bash
git add engine/internal/adapter/gemini/
git commit -m "feat(gemini): skill/agent emulation → GEMINI.md sentinel block + fidelity header"
```

---

### Task 11: Gemini target — MCP into settings.json + golden test

**Files:**
- Modify: `engine/internal/adapter/gemini/gemini.go` (replace `emitMCP` stub)
- Create: `engine/internal/adapter/gemini/testdata/command.golden`, `.../mcp.golden`, `.../skill.golden`
- Test: `engine/internal/adapter/gemini/golden_test.go`

- [ ] **Step 1: Write the failing MCP + golden tests**

`engine/internal/adapter/gemini/golden_test.go`:
```go
package gemini

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

func TestGeminiEmitsMCPSettingsJSON(t *testing.T) {
	a := &model.Artifact{
		Name: "bigquery", Type: model.TypeMCP, Bundle: "stark-data",
		Description: "BQ MCP.", Version: "1.2.0",
		Runtimes: []model.Runtime{model.RuntimeGemini},
		MCP: &model.MCPConfig{
			Transport: "stdio", Command: "stark-bq-mcp",
			Args: []string{"--project", "${BQ_PROJECT}"},
			Env:  map[string]model.SecretRef{"BQ_PROJECT": {SecretRef: "bq-project-id"}},
		},
	}
	files, _, err := New().Render(bundleWith(a))
	if err != nil {
		t.Fatal(err)
	}
	body, ok := find(files, "settings.json")
	if !ok {
		t.Fatalf("expected settings.json; got %v", files)
	}
	for _, want := range []string{`"mcpServers"`, `"bigquery"`, `"command": "stark-bq-mcp"`, `"${BQ_PROJECT}"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("settings.json missing %q in:\n%s", want, body)
		}
	}
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run -update)", name, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenGeminiCommand(t *testing.T) {
	a := &model.Artifact{
		Name: "review", Type: model.TypeCommand, Bundle: "stark-review",
		Description: "PR review command.", Version: "0.7.0",
		ArgumentHint: "[PR_NUMBER]", Body: "Review {{args}}.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, _ := New().Render(bundleWith(a))
	assertGolden(t, "command.golden", files[0].Content)
}

func TestGoldenGeminiSkill(t *testing.T) {
	a := &model.Artifact{
		Name: "stark-review", Type: model.TypeSkill, Bundle: "stark-review",
		Description: "PR review.", Version: "0.7.0",
		Body:     "Review carefully.\n",
		Runtimes: []model.Runtime{model.RuntimeGemini},
	}
	files, _, _ := New().Render(bundleWith(a))
	assertGolden(t, "skill.golden", files[0].Content)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/adapter/gemini/ -run 'TestGeminiEmitsMCP|TestGolden' -v`
Expected: FAIL (`emitMCP not yet implemented`).

- [ ] **Step 3: Implement `emitMCP` (deterministic JSON)**

Replace the `emitMCP` stub in `gemini.go`; add `"bytes"`, `"encoding/json"`:
```go
// geminiSettings mirrors Gemini CLI settings.json mcpServers.<name>. Marshaled
// with encoding/json (object keys sorted by the standard library) for stable
// output (§7.6). One server per fragment; install merges by key (§9.2).
type geminiMCPServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type geminiSettings struct {
	MCPServers map[string]geminiMCPServer `json:"mcpServers"`
}

func (t *Target) emitMCP(a *model.Artifact) ([]adapter.OutputFile, error) {
	if a.MCP == nil {
		return nil, fmt.Errorf("gemini: mcp artifact %q has no mcp config", a.Name)
	}
	srv := geminiMCPServer{Command: a.MCP.Command, Args: a.MCP.Args, URL: a.MCP.URL}
	if len(a.MCP.Env) > 0 {
		srv.Env = map[string]string{}
		for k := range a.MCP.Env {
			srv.Env[k] = "${" + k + "}" // §4.4: never the secret value
		}
	}
	doc := geminiSettings{MCPServers: map[string]geminiMCPServer{a.Name: srv}}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("gemini: marshal settings.json: %w", err)
	}
	return []adapter.OutputFile{{Path: "settings.json", Content: buf.Bytes()}}, nil
}
```
Remove the now-unused `_ = strings.TrimSpace` line from the old stub.

- [ ] **Step 4: Generate goldens, then verify**

Run:
```bash
cd engine && go test ./internal/adapter/gemini/ -run TestGolden -update && go test ./internal/adapter/gemini/ -v && cd ..
```
Expected: goldens written then PASS; `command.golden` contains only `description`/`prompt`; `skill.golden` contains the `EMULATED` header + sentinels.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/adapter/gemini/
git commit -m "feat(gemini): mcp → settings.json mcpServers + byte-exact goldens"
```

---

### Task 12: Cross-artifact aggregator — sentinel merge

**Files:**
- Create: `engine/internal/aggregate/aggregate.go`
- Test: `engine/internal/aggregate/aggregate_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/aggregate/aggregate_test.go`:
```go
package aggregate

import (
	"strings"
	"testing"
)

func TestMergeSortsByBundleName(t *testing.T) {
	secs := []Section{
		{Bundle: "zzz", Name: "b", Content: "B body\n"},
		{Bundle: "aaa", Name: "a", Content: "A body\n"},
	}
	out := Merge(secs)
	ai := strings.Index(out, "stark:begin aaa/a@")
	zi := strings.Index(out, "stark:begin zzz/b@")
	if ai < 0 || zi < 0 || ai > zi {
		t.Fatalf("sections must be sorted by <bundle>/<name>:\n%s", out)
	}
}

func TestMergeWrapsEachSectionInSentinels(t *testing.T) {
	out := Merge([]Section{{Bundle: "stark-review", Name: "x", Content: "hello\n"}})
	if !strings.Contains(out, "<!-- stark:begin stark-review/x@") {
		t.Fatalf("missing begin sentinel:\n%s", out)
	}
	if !strings.Contains(out, "<!-- stark:end stark-review/x -->") {
		t.Fatalf("missing end sentinel:\n%s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("missing content:\n%s", out)
	}
}

func TestMergeEmptyIsEmpty(t *testing.T) {
	if Merge(nil) != "" {
		t.Fatal("empty input must yield empty output")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/aggregate/ -v`
Expected: FAIL — undefined `Section`, `Merge`.

- [ ] **Step 3: Implement the aggregator**

`engine/internal/aggregate/aggregate.go`:
```go
// Package aggregate merges N per-artifact contributions destined for ONE shared
// file (GEMINI.md, AGENTS.md, a single config.toml) into a deterministic,
// sentinel-wrapped document (spec §6.3). Sections are sorted by <bundle>/<name>
// and the merge is idempotent on rebuild. Install merges by sentinel, never
// blind-append (§9.2).
package aggregate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Section is one artifact's contribution to a shared file.
type Section struct {
	Bundle  string
	Name    string
	Content string // inner content (already includes any fidelity header)
}

func (s Section) id() string { return s.Bundle + "/" + s.Name }

func digest(inner string) string {
	sum := sha256.Sum256([]byte(inner))
	return hex.EncodeToString(sum[:])[:12]
}

// Merge wraps each section in stable sentinels, sorts by <bundle>/<name>, and
// concatenates. Pure + deterministic; running it on its own output is a no-op
// modulo re-parse (see property test).
func Merge(sections []Section) string {
	if len(sections) == 0 {
		return ""
	}
	sorted := make([]Section, len(sections))
	copy(sorted, sections)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].id() < sorted[j].id() })

	var b strings.Builder
	for _, s := range sorted {
		inner := s.Content
		if !strings.HasSuffix(inner, "\n") {
			inner += "\n"
		}
		fmt.Fprintf(&b, "<!-- stark:begin %s@%s -->\n", s.id(), digest(inner))
		b.WriteString(inner)
		fmt.Fprintf(&b, "<!-- stark:end %s -->\n", s.id())
	}
	return b.String()
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/aggregate/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/aggregate/
git commit -m "feat(aggregate): sentinel-wrapped shared-file merge, sorted by <bundle>/<name>"
```

---

### Task 13: Aggregator idempotence — parse + property test

**Files:**
- Modify: `engine/internal/aggregate/aggregate.go` (add `Parse`)
- Test: `engine/internal/aggregate/idempotence_test.go`

- [ ] **Step 1: Write the failing property test**

`engine/internal/aggregate/idempotence_test.go`:
```go
package aggregate

import (
	"math/rand"
	"testing"
)

func TestMergeIsIdempotentViaParse(t *testing.T) {
	// Property: Merge(Parse(Merge(S))) == Merge(S) for any section set S,
	// regardless of input order.
	rng := rand.New(rand.NewSource(1))
	for iter := 0; iter < 200; iter++ {
		n := rng.Intn(6)
		secs := make([]Section, n)
		for i := range secs {
			secs[i] = Section{
				Bundle:  string(rune('a' + rng.Intn(4))),
				Name:    string(rune('a' + rng.Intn(4))),
				Content: "body-" + string(rune('a'+rng.Intn(3))) + "\n",
			}
		}
		// dedupe by id (real catalogs have unique ids); keep last writer.
		seen := map[string]Section{}
		var uniq []Section
		for _, s := range secs {
			seen[s.id()] = s
		}
		for _, s := range seen {
			uniq = append(uniq, s)
		}

		first := Merge(uniq)
		reparsed := Parse(first)
		again := Merge(reparsed)
		if first != again {
			t.Fatalf("not idempotent (iter %d):\n--- first ---\n%s\n--- again ---\n%s", iter, first, again)
		}
	}
}

func TestParseRoundTripsSections(t *testing.T) {
	in := []Section{
		{Bundle: "a", Name: "x", Content: "AX\n"},
		{Bundle: "b", Name: "y", Content: "BY\n"},
	}
	parsed := Parse(Merge(in))
	if len(parsed) != 2 {
		t.Fatalf("want 2 sections, got %d", len(parsed))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/aggregate/ -run 'TestMergeIsIdempotent|TestParseRoundTrips' -v`
Expected: FAIL — undefined `Parse`.

- [ ] **Step 3: Implement `Parse`**

Append to `engine/internal/aggregate/aggregate.go`; add `"regexp"`:
```go
var (
	beginRe = regexp.MustCompile(`^<!--\s*stark:begin\s+(\S+?)/(\S+?)@[0-9a-f]+\s*-->$`)
	endRe   = regexp.MustCompile(`^<!--\s*stark:end\s+(\S+?)/(\S+?)\s*-->$`)
)

// Parse extracts the managed sections from a previously-merged document. Content
// outside sentinels is ignored (install preserves it separately). The digest in
// the begin sentinel is dropped — Merge recomputes it — so Parse→Merge is stable.
func Parse(doc string) []Section {
	lines := strings.Split(doc, "\n")
	var out []Section
	var cur *Section
	var buf []string
	for _, ln := range lines {
		if m := beginRe.FindStringSubmatch(ln); m != nil {
			cur = &Section{Bundle: m[1], Name: m[2]}
			buf = nil
			continue
		}
		if m := endRe.FindStringSubmatch(ln); m != nil && cur != nil {
			cur.Content = strings.Join(buf, "\n")
			if cur.Content != "" {
				cur.Content += "\n"
			}
			out = append(out, *cur)
			cur = nil
			continue
		}
		if cur != nil {
			buf = append(buf, ln)
		}
	}
	return out
}
```

> The trailing-newline handling matches `Merge`'s normalization: each inner block
> ends with exactly one `\n`, so reconstructing `strings.Join(buf,"\n")+"\n"`
> reproduces the original inner content byte-for-byte.

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/aggregate/ -v`
Expected: PASS (200 property iterations + round-trip).

- [ ] **Step 5: Commit**

```bash
git add engine/internal/aggregate/
git commit -m "test(aggregate): Parse + idempotence property test (Merge∘Parse∘Merge == Merge)"
```

---

### Task 14: Capability validation — warn emulated / error unsupported

**Files:**
- Create: `engine/internal/validate/rules_capability.go`
- Modify: `engine/internal/validate/validate.go` (call from the artifact loop)
- Test: `engine/internal/validate/capability_test.go`

- [ ] **Step 1: Write the failing test**

`engine/internal/validate/capability_test.go`:
```go
package validate

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestCapabilityWarnsOnEmulated(t *testing.T) {
	// agent on codex is emulated → warning, not error.
	a := &model.Artifact{Name: "rt", Type: model.TypeAgent,
		Runtimes: []model.Runtime{model.RuntimeCodex}}
	r := &Result{}
	checkCapability(r, "demo/agent/rt", a)
	if r.HasErrors() {
		t.Fatalf("emulated should not error: %+v", r.Errors)
	}
	if len(r.Warnings) != 1 {
		t.Fatalf("want 1 emulated warning, got %d", len(r.Warnings))
	}
}

func TestCapabilityErrorsOnUnsupported(t *testing.T) {
	// craft an artifact whose (type,runtime) is unsupported by using an unknown type.
	a := &model.Artifact{Name: "x", Type: model.ArtifactType("widget"),
		Runtimes: []model.Runtime{model.RuntimeGemini}}
	r := &Result{}
	checkCapability(r, "demo/widget/x", a)
	if !r.HasErrors() {
		t.Fatal("unsupported (type,runtime) must error")
	}
}

func TestCapabilityNativeIsSilent(t *testing.T) {
	a := &model.Artifact{Name: "s", Type: model.TypeSkill,
		Runtimes: []model.Runtime{model.RuntimeClaude}}
	r := &Result{}
	checkCapability(r, "demo/skill/s", a)
	if r.HasErrors() || len(r.Warnings) != 0 {
		t.Fatalf("native should be silent: %+v / %+v", r.Errors, r.Warnings)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/validate/ -run TestCapability -v`
Expected: FAIL — undefined `checkCapability`.

- [ ] **Step 3: Implement the rule**

`engine/internal/validate/rules_capability.go`:
```go
package validate

import (
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/capability"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// checkCapability enforces the §6 matrix: warn on emulated targets (counted +
// surfaced), error on unsupported ones (spec §7.4).
func checkCapability(r *Result, where string, a *model.Artifact) {
	for _, rt := range a.Runtimes {
		switch capability.Level(a.Type, rt) {
		case model.SupportEmulated:
			r.Warnf(where, "%s on %s is emulated — verify fidelity (§6.1)", a.Type, rt)
		case model.SupportUnsupported:
			r.Errorf(where, "%s on %s is unsupported", a.Type, rt)
		}
	}
}
```

Add the call inside `Catalog()`'s artifact loop in `validate.go`, after `checkFences`:
```go
			checkCapability(r, where, a)
```

- [ ] **Step 4: Run to verify pass (and confirm seed catalog still valid)**

Run: `cd engine && go test ./internal/validate/ -v`
Expected: PASS. Note: the slice-01 seed (`stark-gh`, command + mcp on all 3 runtimes) is all native → no new errors. If a future emulated seed artifact lands, `TestSeedCatalogIsValid` (checks `HasErrors`, not warnings) stays green.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/validate/
git commit -m "feat(validate): capability rule — warn emulated, error unsupported (§6)"
```

---

### Task 15: Three-runtime build integration + determinism

**Files:**
- Create: `engine/internal/adapter/buildall_test.go`

- [ ] **Step 1: Write the integration test (all 3 targets over the seed catalog)**

`engine/internal/adapter/buildall_test.go`:
```go
package adapter_test

import (
	"sort"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/codex"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/gemini"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/load"
)

// emitAll runs the canonical bundle-level Render (CC-1) over every catalog bundle.
// The target resolves bodies via merge.Resolve internally — the caller does no
// fence.Strip and passes no body.
func emitAll(t *testing.T, tgt adapter.Target) map[string]string {
	t.Helper()
	cat, err := load.Load("../../../catalog")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, b := range cat.Bundles {
		files, _, err := tgt.Render(b)
		if err != nil {
			t.Fatalf("%s render on %s: %v", b.Name, tgt.Runtime(), err)
		}
		for _, f := range files {
			out[f.Path] += string(f.Content) // shared files accumulate
		}
	}
	return out
}

func TestThreeRuntimeEmitDeterministic(t *testing.T) {
	for _, tgt := range []adapter.Target{codex.New(), gemini.New()} {
		first := emitAll(t, tgt)
		second := emitAll(t, tgt)
		if len(first) != len(second) {
			t.Fatalf("%s: file count differs", tgt.Runtime())
		}
		paths := make([]string, 0, len(first))
		for p := range first {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if first[p] != second[p] {
				t.Fatalf("%s: %s not deterministic", tgt.Runtime(), p)
			}
		}
	}
}

func TestCodexEmitsSomethingForSeed(t *testing.T) {
	out := emitAll(t, codex.New())
	if len(out) == 0 {
		t.Fatal("codex produced no output for the seed catalog")
	}
}
```

- [ ] **Step 2: Run to verify it fails (then passes)**

Run: `cd engine && go test ./internal/adapter/ -run 'TestThreeRuntime|TestCodexEmitsSomething' -v`
Expected: PASS (targets already implemented). If the seed `stark-gh` command + mcp emit cleanly on codex/gemini, both subtests pass. This is the build-twice determinism guard (§7.6) for the new targets.

- [ ] **Step 3: Run the full engine suite**

Run: `cd engine && go test ./... && cd ..`
Expected: PASS across model, fence, load, validate, fieldmap, aggregate, emulate, capability, codex, gemini, adapter.

- [ ] **Step 4: Commit**

```bash
git add engine/internal/adapter/buildall_test.go
git commit -m "test(adapter): 3-runtime emit over seed catalog + build-twice determinism"
```

---

### Task 16: Per-target version surfacing + docs

**Files:**
- Create: `engine/internal/adapter/registry.go`
- Test: `engine/internal/adapter/registry_test.go`
- Modify: `docs/` capability note (or repo README section) — record target versions

- [ ] **Step 1: Write the failing test**

`engine/internal/adapter/registry_test.go`:
```go
package adapter_test

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func TestRegistryHasAllNonClaudeTargets(t *testing.T) {
	reg := adapter.Targets()
	if _, ok := reg[model.RuntimeCodex]; !ok {
		t.Fatal("codex target not registered")
	}
	if _, ok := reg[model.RuntimeGemini]; !ok {
		t.Fatal("gemini target not registered")
	}
	// versions are independently namespaced (§7.7).
	if reg[model.RuntimeCodex].Version() == reg[model.RuntimeGemini].Version() {
		t.Fatal("targets must carry distinct version identities")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/adapter/ -run TestRegistry -v`
Expected: FAIL — undefined `Targets`.

- [ ] **Step 3: Implement the registry**

`engine/internal/adapter/registry.go`:
```go
package adapter

import (
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/codex"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/gemini"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// Targets returns the non-Claude runtime targets keyed by runtime. The Claude
// target is registered by plan 02; build wiring (plan 04+) merges both maps.
// Each target is independently versioned (spec §7.7) so a format fix to one
// runtime churns only that runtime's output.
func Targets() map[model.Runtime]Target {
	return map[model.Runtime]Target{
		model.RuntimeCodex:  codex.New(),
		model.RuntimeGemini: gemini.New(),
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/adapter/ -v`
Expected: PASS.

- [ ] **Step 5: Record target versions in docs**

Append a short note to `docs/superpowers/` (or the repo README under a "Adapter target versions" heading): codex@1 (native skills at `.agents/skills/<name>/SKILL.md`; mcp at `config.toml [mcp_servers.<name>]`), gemini@1 (commands at `.gemini/commands/<name>.toml`; skill/agent emulated in `GEMINI.md`; mcp in `settings.json`). State that bumping a target version is its own PR type (§7.7) and that `dist/codex/` + `dist/gemini/` are **not committed** (§5.1) — built on `stark install`.

- [ ] **Step 6: Commit**

```bash
git add engine/internal/adapter/registry.go engine/internal/adapter/registry_test.go docs/
git commit -m "feat(adapter): non-Claude target registry + per-target version docs (§7.7)"
```

---

### Task 17: outputNamespace — fold Codex `TypeAgent` into the `skilllike` bucket (CC-7)

Plan 01's `engine/internal/validate/rules_structural.go` `outputNamespace` decides which
artifacts collide by computing the runtime-relative output namespace. On Codex,
skill/prompt/command **and agent** all emit `.agents/skills/<name>/SKILL.md` (Task 6's
`emitSkill`), so an agent and a skill with the same `<name>` would silently overwrite each
other. This task folds `TypeAgent` into the same `codex:skilllike` bucket as
skill/prompt/command so the structural collision rule catches it.

**Files:**
- Modify: `engine/internal/validate/rules_structural.go` (plan 01's `outputNamespace`)
- Test: `engine/internal/validate/rules_structural_test.go` (add a Codex skill+agent case)

- [ ] **Step 1: Write the failing test**

Append to `engine/internal/validate/rules_structural_test.go`:
```go
func TestCodexAgentCollidesWithSkill(t *testing.T) {
	// On Codex, skill "x" and agent "x" both emit .agents/skills/x/SKILL.md →
	// they share one namespace and MUST be reported as a collision (CC-7).
	skill := &model.Artifact{
		Name: "x", Type: model.TypeSkill, Bundle: "demo",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	agent := &model.Artifact{
		Name: "x", Type: model.TypeAgent, Bundle: "demo",
		Runtimes: []model.Runtime{model.RuntimeCodex},
	}
	if got, want := outputNamespace(skill, model.RuntimeCodex),
		outputNamespace(agent, model.RuntimeCodex); got != want {
		t.Fatalf("codex skill and agent must share a namespace: %q vs %q", got, want)
	}

	r := &Result{}
	checkOutputCollisions(r, "demo", []*model.Artifact{skill, agent})
	if !r.HasErrors() {
		t.Fatal("codex skill+agent name collision must error")
	}
}

func TestCodexAgentDistinctFromGeminiSkill(t *testing.T) {
	// Guard: the fold is Codex-only. On Gemini, skill/agent emulate into GEMINI.md
	// sentinel blocks (no filename collision) so they keep distinct namespaces.
	skill := &model.Artifact{Name: "x", Type: model.TypeSkill, Bundle: "demo"}
	agent := &model.Artifact{Name: "x", Type: model.TypeAgent, Bundle: "demo"}
	if outputNamespace(skill, model.RuntimeGemini) == outputNamespace(agent, model.RuntimeGemini) {
		t.Fatal("gemini skill/agent emulate to sentinel blocks; must NOT share a namespace")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/validate/ -run 'TestCodexAgent' -v`
Expected: FAIL — `outputNamespace(agent, RuntimeCodex)` still returns an agent-specific
bucket, so the namespaces differ and no collision is reported.

- [ ] **Step 3: Implement the fold**

In `engine/internal/validate/rules_structural.go`, update `outputNamespace` so that on
Codex, `TypeAgent` joins skill/prompt/command in the shared `skilllike` bucket (matching
Task 6, where all four emit `.agents/skills/<name>/SKILL.md`):
```go
// outputNamespace returns the runtime-relative collision key for an artifact:
// two artifacts that resolve to the same namespace would overwrite each other in
// that runtime's dist tree.
func outputNamespace(a *model.Artifact, rt model.Runtime) string {
	switch rt {
	case model.RuntimeCodex:
		// Codex: skill/prompt/command AND agent all emit
		// .agents/skills/<name>/SKILL.md (agent is emulated as a skill) → one bucket.
		switch a.Type {
		case model.TypeSkill, model.TypePrompt, model.TypeCommand, model.TypeAgent:
			return "codex:skilllike/" + a.Name
		case model.TypeMCP:
			return "codex:mcp/" + a.Name
		}
	case model.RuntimeGemini:
		// Gemini: prompt/command → one .gemini/commands namespace; skill/agent
		// emulate into GEMINI.md sentinel blocks keyed by <bundle>/<name> (no
		// filename collision) so they keep type-distinct namespaces.
		switch a.Type {
		case model.TypePrompt, model.TypeCommand:
			return "gemini:command/" + a.Name
		case model.TypeSkill:
			return "gemini:skill/" + a.Name
		case model.TypeAgent:
			return "gemini:agent/" + a.Name
		case model.TypeMCP:
			return "gemini:mcp/" + a.Name
		}
	}
	// Claude (and default): every type keeps its own namespace.
	return string(rt) + ":" + string(a.Type) + "/" + a.Name
}
```
> Keep the rest of `rules_structural.go` (the `checkOutputCollisions` loop that calls
> `outputNamespace` per targeted runtime and `Errorf`s on a duplicate key) unchanged — only
> the Codex `TypeAgent` arm moves. If plan 01's `outputNamespace` lacks a per-runtime
> `switch`, refactor it to the shape above first (its existing Claude/Gemini behavior is
> preserved verbatim).

- [ ] **Step 4: Run to verify pass (and no regression)**

Run: `cd engine && go test ./internal/validate/ -v`
Expected: PASS — the Codex skill+agent collision is now caught; the Gemini guard and the
existing structural tests stay green.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/validate/rules_structural.go engine/internal/validate/rules_structural_test.go
git commit -m "fix(validate): fold Codex agent into skilllike namespace so name collisions are caught (CC-7)"
```

---

### Task 18: Wire support + enrich index/detail for all runtimes, re-commit (CC-4)

Plan 02 emits the lean `index.json` with `support` populated for **claude only** and the
`bundles/<name>.json` detail with empty `support`/`outputs`/`fidelityNotes`. Now that the
Codex and Gemini targets exist, this task (a) re-wires `index.supportFor` to consult
`capability.Level(type, runtime)` for **all** targeted runtimes, (b) enriches each detail
artifact with per-runtime `support`, `outputs` (path/kind/key/sentinel/emulated derived from
each target's `Render`), and `fidelityNotes` (from the emulation fidelity header /
capability), then (c) runs `stark build --fix` and re-commits the regenerated `index.json` +
`bundles/*.json` as a reviewed diff, locked by a golden.

**Files:**
- Modify: `engine/internal/index/index.go` (plan 02's `supportFor` / index builder)
- Modify: `engine/internal/index/detail.go` (plan 02's detail builder — fill `support`/`outputs`/`fidelityNotes`)
- Test: `engine/internal/index/support_test.go`
- Regenerate + commit: `index.json`, `bundles/*.json`

- [ ] **Step 1: Write the failing test**

`engine/internal/index/support_test.go`:
```go
package index

import (
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// targetedRuntimes is the set of runtimes the engine builds for (claude+codex+gemini).
var targetedRuntimes = []model.Runtime{
	model.RuntimeClaude, model.RuntimeCodex, model.RuntimeGemini,
}

func TestSupportForCoversAllTargetedRuntimes(t *testing.T) {
	a := &model.Artifact{
		Name: "red-team", Type: model.TypeAgent, Bundle: "stark-review",
		Runtimes: targetedRuntimes,
	}
	sup := supportFor(a, targetedRuntimes)
	for _, rt := range targetedRuntimes {
		if _, ok := sup[rt]; !ok {
			t.Fatalf("support map missing runtime %q (claude-only regression)", rt)
		}
	}
	// agent: native on claude, emulated on codex AND gemini (capability matrix §6).
	if sup[model.RuntimeClaude] != model.SupportNative {
		t.Fatalf("claude agent should be native, got %q", sup[model.RuntimeClaude])
	}
	if sup[model.RuntimeCodex] != model.SupportEmulated {
		t.Fatalf("codex agent should be emulated, got %q", sup[model.RuntimeCodex])
	}
	if sup[model.RuntimeGemini] != model.SupportEmulated {
		t.Fatalf("gemini agent should be emulated, got %q", sup[model.RuntimeGemini])
	}
}

func TestDetailArtifactOutputsAndFidelityPopulated(t *testing.T) {
	a := &model.Artifact{
		Name: "red-team", Type: model.TypeAgent, Bundle: "stark-review",
		Description: "Adversarial reviewer.", Body: "Attack the design.\n",
		Runtimes: targetedRuntimes,
	}
	da := buildDetailArtifact(a, targetedRuntimes)
	// outputs has an entry per targeted runtime, each with at least one file.
	for _, rt := range targetedRuntimes {
		outs, ok := da.Outputs[rt]
		if !ok || len(outs) == 0 {
			t.Fatalf("outputs missing for runtime %q: %+v", rt, da.Outputs)
		}
		if outs[0].Path == "" || outs[0].Kind == "" {
			t.Fatalf("output for %q missing path/kind: %+v", rt, outs[0])
		}
	}
	// emulated runtimes carry a fidelity note; native ones do not.
	if da.FidelityNotes[model.RuntimeCodex] == "" {
		t.Fatal("codex agent (emulated) must carry a fidelityNote")
	}
	if _, ok := da.FidelityNotes[model.RuntimeClaude]; ok {
		t.Fatal("claude agent (native) must NOT carry a fidelityNote")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd engine && go test ./internal/index/ -run 'TestSupportForCovers|TestDetailArtifactOutputs' -v`
Expected: FAIL — `supportFor` only populates claude; `buildDetailArtifact` leaves
`Outputs`/`FidelityNotes` empty (undefined or nil per plan 02's skeleton).

- [ ] **Step 3: Implement the wiring + enrichment**

In `engine/internal/index/index.go`, replace the claude-only `supportFor` with a
matrix-driven version over all targeted runtimes:
```go
import "github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/capability"

// supportFor returns the per-runtime support level for an artifact across every
// targeted runtime (CC-4), sourced from the versioned capability matrix (§6).
func supportFor(a *model.Artifact, targeted []model.Runtime) map[model.Runtime]model.SupportLevel {
	out := make(map[model.Runtime]model.SupportLevel, len(targeted))
	for _, rt := range targeted {
		if !targetsRuntime(a, rt) {
			continue // artifact does not opt into this runtime
		}
		out[rt] = capability.Level(a.Type, rt)
	}
	return out
}
```

In `engine/internal/index/detail.go`, fill the detail artifact's `Outputs` and
`FidelityNotes` by rendering each targeted runtime's target and recording its output files:
```go
import (
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/adapter/capability"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// Output is one emitted-file descriptor surfaced in bundles/<name>.json (CC-3).
type Output struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"` // file | mergeJSONKey | mergeTOMLKey | sentinel
	Key      string `json:"key,omitempty"`
	Sentinel string `json:"sentinel,omitempty"`
	Emulated bool   `json:"emulated"`
}

// buildDetailArtifact enriches a detail artifact with per-runtime support,
// outputs, and fidelity notes (CC-4). targets is the registry from adapter.Targets()
// plus the claude target; here we read it via the shared adapter registry.
func buildDetailArtifact(a *model.Artifact, targeted []model.Runtime) DetailArtifact {
	da := DetailArtifact{
		Name: a.Name, Type: a.Type, Description: a.Description,
		Version: a.Version, Runtimes: a.Runtimes,
		Support:       supportFor(a, targeted),
		Outputs:       map[model.Runtime][]Output{},
		FidelityNotes: map[model.Runtime]string{},
	}
	reg := adapter.AllTargets() // claude + codex + gemini, keyed by runtime
	for _, rt := range targeted {
		if !targetsRuntime(a, rt) {
			continue
		}
		lvl := capability.Level(a.Type, rt)
		tgt, ok := reg[rt]
		if !ok {
			continue
		}
		files, _, err := tgt.Render(&model.Bundle{Name: a.Bundle, Artifacts: []*model.Artifact{a}})
		if err != nil {
			continue // build verb surfaces the hard error; detail records what rendered
		}
		for _, f := range files {
			da.Outputs[rt] = append(da.Outputs[rt], Output{
				Path:     f.Path,
				Kind:     outputKind(f.Path),
				Emulated: lvl == model.SupportEmulated,
			})
		}
		if lvl == model.SupportEmulated {
			da.FidelityNotes[rt] = "emulated — derived shape; may not auto-activate on this runtime; verify."
		}
	}
	return da
}

// outputKind classifies an emitted path into the CC-3 kind vocabulary.
func outputKind(path string) string {
	switch {
	case strings.HasSuffix(path, "config.toml"):
		return "mergeTOMLKey"
	case strings.HasSuffix(path, "settings.json"):
		return "mergeJSONKey"
	case strings.HasSuffix(path, "GEMINI.md"), strings.HasSuffix(path, "AGENTS.md"):
		return "sentinel"
	default:
		return "file"
	}
}

func targetsRuntime(a *model.Artifact, rt model.Runtime) bool {
	for _, r := range a.Runtimes {
		if r == rt {
			return true
		}
	}
	return false
}
```
> `adapter.AllTargets()` is the union of the claude target (plan 02) and the non-claude
> registry (`adapter.Targets()`, Task 16). If plan 02 has not yet exported a combined
> accessor, add `AllTargets()` in `engine/internal/adapter/registry.go` merging the claude
> target into the Task-16 map; it is the single place build wiring already merges both.

- [ ] **Step 4: Run to verify pass**

Run: `cd engine && go test ./internal/index/ -v`
Expected: PASS — `support` covers all targeted runtimes; `outputs`/`fidelityNotes`
populated per runtime; emulated runtimes carry a note, native ones do not.

- [ ] **Step 5: Regenerate index + detail and re-commit as a reviewed diff**

Run:
```bash
cd engine && go run ./cmd/stark build --fix && cd ..
git diff --stat dist/claude/index.json dist/claude/bundles/
```
Expected: `dist/claude/index.json` now carries `support` for every targeted runtime per
artifact (not just claude); each `dist/claude/bundles/<name>.json` artifact gains populated
`support`, `outputs`, and `fidelityNotes`. Review the diff (it is the canonical, committed
Claude dist — codex/gemini dist stay in-memory and uncommitted).

- [ ] **Step 6: Lock with a golden over the regenerated index**

`engine/internal/index/support_golden_test.go`:
```go
package index

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

// TestCommittedIndexSupportFullyPopulated asserts the committed dist index has a
// non-empty support level for EVERY targeted runtime an artifact opts into (CC-4),
// guarding against a claude-only regression.
func TestCommittedIndexSupportFullyPopulated(t *testing.T) {
	raw, err := os.ReadFile("../../../dist/claude/index.json")
	if err != nil {
		t.Fatal(err)
	}
	var idx struct {
		Artifacts []struct {
			Name     string                              `json:"name"`
			Runtimes []model.Runtime                     `json:"runtimes"`
			Support  map[model.Runtime]model.SupportLevel `json:"support"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatal(err)
	}
	for _, a := range idx.Artifacts {
		for _, rt := range a.Runtimes {
			lvl, ok := a.Support[rt]
			if !ok || lvl == "" {
				t.Fatalf("artifact %q missing support for targeted runtime %q", a.Name, rt)
			}
		}
	}
}
```
Run: `cd engine && go test ./internal/index/ -run TestCommittedIndexSupportFullyPopulated -v`
Expected: PASS against the regenerated, committed index.

- [ ] **Step 7: Commit**

```bash
git add engine/internal/index/ dist/claude/index.json dist/claude/bundles/
git commit -m "feat(index): populate support/outputs/fidelityNotes for all runtimes + re-commit dist (CC-4)"
```

---

## Self-Review (completed during authoring)

- **Spec coverage (slice 3 scope = spec §16 step 3):**
  - Codex target — native Skills `.agents/skills/<name>/SKILL.md` (name+description required), prompt/command→skill, agent→emulated skill, mcp→`config.toml [mcp_servers.<name>]` ✓ (Tasks 6–8). Open-q §15.1 (Codex MCP key) **pinned** in Task 7 + goldens.
  - Gemini target — command/prompt→`.gemini/commands/<name>.toml` (prompt+description only, `{{args}}`), skill/agent→`GEMINI.md` sentinel blocks, mcp→`settings.json mcpServers.<name>` ✓ (Tasks 9–11). Gemini Extensions noted as §15.2 open question, non-blocking (gemini@2 path) ✓.
  - Per-field fallback (carry/map/drop+warn/derive/best-effort) for model, argument-hint, disable-model-invocation, allowed-tools/tools ✓ (Tasks 4–5, §6.2 contract table).
  - Emulation fidelity header on every emulated output ✓ (Task 3; applied in codex agent + gemini skill/agent).
  - Cross-artifact aggregation — sentinel-wrapped, sorted by `<bundle>/<name>`, idempotent + property test ✓ (Tasks 12–13).
  - Capability matrix as versioned data; support levels surfaced; warn emulated / error unsupported ✓ (Tasks 2, 14).
  - Per-target independent versioning ✓ (codex@1/gemini@1, Tasks 6/9/16).
  - **Canonical adapter interface (CC-1):** both targets implement the bundle-level
    `Render(b *model.Bundle) ([]adapter.OutputFile, []adapter.Finding, error)` — no alternate
    artifact-level `Emit(a, body)` interface; each target iterates `b.Artifacts` and calls
    `merge.Resolve(a, rt)` (which runs `fence.Strip`) internally to get the resolved body ✓
    (Tasks 1, 6, 9 + all target/golden/integration tests). Requires plan 02 merged first.
  - **Codex outputNamespace agent-fold (CC-7):** plan 01's `outputNamespace` folds Codex
    `TypeAgent` into the shared `codex:skilllike` bucket so a Codex skill "x" + agent "x"
    name collision is caught; Gemini skill/agent stay distinct (sentinel blocks) ✓ (Task 17).
  - **Support/detail enrichment for all runtimes (CC-4):** `index.supportFor` consults
    `capability.Level` for every targeted runtime (not claude-only); detail artifacts gain
    per-runtime `support`, `outputs` (path/kind/key/sentinel/emulated from each target's
    `Render`), and `fidelityNotes`; `stark build --fix` regenerates and re-commits
    `dist/claude/index.json` + `dist/claude/bundles/*.json` as a reviewed diff, locked by a
    golden asserting `support` is fully populated per targeted runtime ✓ (Task 18).
  - `dist/codex` + `dist/gemini` **not committed** — no commit step writes them; only in-memory
    emit + goldens (§5.1). Task 18's re-commit touches only the canonical committed `dist/claude`.
- **Type consistency:** uses `model.Artifact/Bundle/Runtime/ArtifactType/SupportLevel/MCPConfig/SecretRef`, `merge.Resolve`, `fence.Strip` (inside `merge.Resolve`), `load.Load`, `capability.Level`, `validate.Result/Errorf/Warnf/HasErrors`, and the plan-02 canonical `adapter.Target`/`OutputFile`/`Finding` verbatim. New exported names (`capability.Level/Version`, `fieldmap.Apply/Action/Result`, `aggregate.Merge/Parse/Section`, `emulate.Header`, `codex.New`, `gemini.New`, `adapter.Targets`/`AllTargets`, `index.Output`) are introduced here for later slices.
- **TOML/JSON determinism:** Codex/Gemini command TOML from ordered structs; MCP maps are single-key + alphabetized env; JSON via `encoding/json` (sorted keys). No reliance on Go map iteration order in emitted bytes.
- **Placeholder scan:** none — every step has complete runnable Go + exact run/expected-FAIL/expected-PASS/commit. Stubs in Tasks 6 and 9 are explicitly replaced in Tasks 7 and 10/11 within this same plan.

---

## Execution handoff

This is slice 3 of 8 (see spec §16). **Requires plan 02 merged first** — it depends on slice 1 (model/loader/validate) and the slice-02 canonical adapter interface (CC-1: `adapter.Target`/`OutputFile`/`Finding` + `merge.Resolve`) and the plan-02 `internal/index` builder (extended by Task 18). Recommended execution: **subagent-driven-development**, one subagent per task, review between tasks. Golden files are regenerated only via `-update` and reviewed as format changes (§7.7). Next: slice 4 (CC marketplace generator) consumes these targets' output paths; slice 5 (CLI install) consumes `aggregate.Parse`/`Merge` + the per-target versions for safe shared-file merges.
