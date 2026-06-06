# stark-marketplace plans — red-team resolutions & canonical contracts

**Date:** 2026-06-06 · Three parallel adversarial reviews (cross-plan consistency, Go
executability, spec-coverage/sequencing) against plans 01–08. This doc records the
**canonical contracts** every plan must conform to (the parallel-authoring drift fixes)
and the per-plan resolutions folded into the plan files.

## Canonical contracts (single source of truth)

### CC-1 · Adapter interface (owner: plan 02, package `internal/adapter`)
```go
type OutputFile struct { Path string; Content []byte }
type Finding    struct { Where string; Level string; Msg string } // Level: "warn"|"error"
type Target interface {
    Runtime() model.Runtime
    Version() string
    Render(b *model.Bundle) ([]OutputFile, []Finding, error) // bundle-level
}
```
Targets iterate `b.Artifacts`, call `merge.Resolve(a, rt)` (which runs `fence.Strip`)
**internally** — they do NOT receive a pre-stripped `body`. Plans 03/04/05 use this exact
signature. Plan 03's alternate artifact-level `Emit` interface is removed.

### CC-2 · `index.json` (lean, owner: plan 02 `internal/index`)
Top-level key is **`artifacts`** (not `entries`). `schemaVersion` is an **int**, currently
`1`, declared once in `internal/index` and referenced by consumers.
```jsonc
{ "schemaVersion": 1, "generatedBy": {"adapterVersions": {...}},
  "artifacts": [ {
    "name","type","bundle","description","tags","category","maturity",
    "version","runtimes":["claude",...],
    "support": {"claude":"native","codex":"native","gemini":"emulated"},
    "digest":"sha256:…"
  } ] }
```
No bundle-type rows in the lean index — bundle metadata lives in `bundles/<name>.json`.
Plan 05's `indexio` and plan 06's TS types use key `artifacts` and field `description`.

### CC-3 · `bundles/<name>.json` (detail, owner: plan 02 builder + plan 03 enrichment)
```jsonc
{ "schemaVersion": 1,
  "bundle": {"name","version","description","category","tags","owner","maturity","homepage"},
  "artifacts": [ {
    "name","type","description","version","runtimes",
    "support": {"<runtime>":"native|emulated|unsupported"},
    "requires": [{"type","ref"}],
    "diverged": false,
    "outputs": {"<runtime>": [ {"path","kind","key","sentinel","emulated"} ]},
    "fidelityNotes": {"<runtime>":"…"}
  } ] }
```
`kind` ∈ `file | mergeJSONKey | mergeTOMLKey | sentinel`. Plan 05 install drives off
`outputs[]`. Plan 06 web derives display `outputPaths[rt]` = first `outputs[rt][].path`.
The detail is **engine-emitted**, never hand-shaped per consumer; plan 06 fixtures are
generated from `stark build` output (or kept byte-aligned to it + asserted).

### CC-4 · Support population (owner: plan 03)
`index.supportFor` and the detail `support`/`outputs`/`fidelityNotes` are populated from
`capability.Level(type, runtime)` + each target's render for **all** targeted runtimes.
Plan 03 adds an explicit task to (a) re-wire `internal/index`, (b) `stark build --fix` and
**re-commit** the regenerated `index.json` + `bundles/*.json` as a reviewed diff, (c) lock
with a golden asserting `support` is fully populated for every targeted runtime.

### CC-5 · Version-bump immutability gate (owner: plan 02 verb + plan 08 CI)
Plan 02 adds `stark check-bumps`: load previous `index.json` (from `origin/main`/`HEAD`),
recompute `digest.Source()` (display-metadata-excluded canonical-source hash) per artifact,
**error** when a source digest changed but `version` did not. Empty previous index = skip.
Plan 08 wires it as a **required, blocking** CI step. Plan 02 self-review's "§11 ✓" is
corrected to point at this gate, not just the hash primitive.

### CC-6 · `runBuild` signature (owner: plan 02)
`func runBuild(catalogDir, repoRoot string, check bool) int`. Plan 04 references this exact
signature and the already-loaded `cat` inside it.

### CC-7 · Output-namespace collision model (owner: plan 01 model, sync task in plan 03)
On Codex, skill/prompt/command/agent ALL emit `.agents/skills/<name>/SKILL.md`, so they
share one namespace. Plan 03 adds a task updating plan 01's `outputNamespace` to fold
`TypeAgent` into the Codex `skilllike` bucket (alongside skill/prompt/command). Gemini:
prompt/command → one `command` namespace; skill/agent → sentinel blocks (no filename
collision).

## Per-plan resolutions

### Plan 01 (foundation)
- **F-Go#1/#3/#4 — schema `$ref` + embed:** drop the `stark:requires` `$ref`; **inline** the
  `requires` subschema into each artifact schema. Author schemas **only** under
  `engine/internal/validate/schema/` (the embed root); root `schema/` becomes a generated
  copy emitted by the build + a drift test (no dual hand-edited source). Reorder Task 4 so
  the dir + JSON files exist before `schema.go` is written / any `go` command runs. Stop
  discarding `AddResource`/`Compile` errors.
- **F-Go#5 — `Artifact.Raw`:** don't discard the second decode error (record a finding);
  obtain the schema-validation instance via YAML→JSON round-trip so number/type fidelity
  matches `jsonschema/v6`.
- **F-Go#9 — `scanInlineCred` precedence:** parenthesize per-pattern; add tests for `key=`
  and `--token=` without `@`.
- **allowlist:** remove `python3` (workspace No-Python + minimal-allowlist intent).
- **outputNamespace:** keep as-is here; plan 03 owns the Codex-agent fold (CC-7).
- **.gitignore:** add `dist/codex/` + `dist/gemini/`.

### Plan 02 (engine core)
- Adopt CC-1 interface verbatim; CC-2/CC-3 JSON shapes (add `description` to lean Entry;
  emit detail skeleton with `support`/`outputs`/`fidelityNotes` fields, claude-populated,
  others filled by plan 03). CC-6 signature. Add `stark check-bumps` (CC-5).
- **F-Cov#7 / F-Go write-side:** normalize every `OutputFile.Content` to LF in
  `build.Write`; test asserts no `\r` in generated files.
- **F-Cov#9 — frontmatter booleans:** emit booleans present in the resolved frontmatter map
  explicitly instead of blanket-omitting `false`; test `disable-model-invocation: false`
  survives.
- Correct self-review: "§11 ✓" only once `check-bumps` exists.

### Plan 03 (multi-runtime)
- Remove the alternate `Emit` interface; implement CC-1 `Render`. Add tasks: (a) wire
  `index.supportFor` + detail enrichment to `capability.Level` and re-commit regenerated
  index/detail with goldens (CC-4); (b) update `outputNamespace` for the Codex-agent fold
  (CC-7). Keep codex/gemini dist in-memory (uncommitted).

### Plan 04 (cc-marketplace)
- Use CC-6 `runBuild(catalogDir, repoRoot, check)` and the in-scope `cat`. Marketplace
  output stays inside committed `dist/claude` (covered by the existing drift gate).

### Plan 05 (cli)
- index key `artifacts` (CC-2); consume CC-3 `outputs[]`. Drop dependency on lean-index
  bundle rows (use `bundles/<name>.json`). **Add a task** implementing the §9.5
  authenticated GitHub-API fetch transport behind `indexio.LoadIndex`/`LoadBundleDetail`
  (token from `gh`/env; no anonymous raw URL) — no longer deferred.
- **F-Go#7/#8 — TOML splice:** tolerant header parse (whitespace/quoted keys); when ending a
  managed table, only stop at a `[` that is NOT a subtable of the dotted key; assert the
  managed payload contains no `[header]` lines before splicing. Add round-trip tests with a
  pre-existing `[mcp_servers]` parent + a `[mcp_servers.<n>.sub]` subtable.
- **F-Go#10/#11 — sequencing:** merge info/closure/consent task ordering so each step's
  package compiles; correct the "expected FAIL" reasons to cite the real cross-package
  undefined symbol, or add minimal stubs.

### Plan 06 (web-registry)
- TS index type uses key `artifacts` + `description` (CC-2). Detail type reads CC-3
  `outputs[]` and derives `outputPaths`; align `__fixtures__` to the engine-emitted shape
  (generate from `stark build` or assert byte-alignment). Add a defensive test:
  `SupportBadges` renders a partially-populated `support` map without crashing.

### Plan 08 (security-hardening)
- Wire `stark check-bumps` as a required, blocking CI step (CC-5). Add
  `engine/internal/validate/allowlist.go` to a CODEOWNERS sensitive path. Make the
  branch-protection note explicit: high-trust body paths require **2 approvals**
  (CODEOWNERS + required-review-count), not a single CODEOWNERS entry. Reference the
  allowlist file + governance in `docs/SECURITY.md`.

### Plan 07 (migration)
- No findings. Unchanged (already reuses plan-01 anchors + validates imported output).
