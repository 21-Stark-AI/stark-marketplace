# stark-marketplace — Design Spec (v2, post red-team)

**Status:** Draft (autopilot) · **Date:** 2026-06-06 · **Owner:** Aryeh Kiovetsky
**Audience:** Evinced engineering (internal)
**Changelog:** v2 incorporates the four-lens red-team
(`2026-06-06-stark-marketplace-redteam.md`).

---

## 1. Summary

`stark-marketplace` is an **Evinced-internal marketplace** for sharing reusable agent
artifacts — **skills, prompts, slash commands, agents, and MCP servers** — across the
three agent runtimes Evinced uses: **Claude Code, Codex (OpenAI), and Gemini CLI**.

Each artifact is authored **once** in a canonical, runtime-agnostic format. A
deterministic Go build pipeline transpiles it into each runtime's native shape, produces
a searchable index, and emits a native Claude Code marketplace manifest so artifacts
install through `/plugin marketplace add` with no custom client. The repository **is** the
catalog (monorepo). Discovery happens through three surfaces: the native CC marketplace, a
CLI (`stark`), and an SSO-gated internal web registry.

> **Security posture up front (this is a code-distribution system, not just config):**
> every artifact body is instruction text injected into a developer's agent, and every
> MCP server is a command line spawned on the developer's machine. The design treats
> artifact bodies and MCP commands as the highest-trust surfaces — see §7.4. Integrity
> rests on protected git history + a CI-signed build manifest, **not** on self-computed
> digests (§7.5, §11).

### Goals

1. **Author once, run on three runtimes** with correct native output per runtime.
2. **Robust, reproducible infrastructure** — deterministic builds (pinned toolchain),
   CI-enforced validation + drift detection, signed provenance, no hand-edited output.
3. **Low authoring friction** — "write one markdown file" for ~90% of cases; per-runtime
   divergence is an explicit, budgeted, reviewable escape hatch.
4. **Safe install** — idempotent, atomic, reversible installs that never silently clobber
   a developer's existing config; explicit consent for code-executing artifact classes.
5. **Native install paths** — Claude Code installs natively; CLI + web give parity for
   Codex/Gemini and for browsing/discovery.
6. **Clean migration** of existing `stark-skills` artifacts with minimal rewrites.

### Non-goals ("what this is not")

- **Not public/open.** Evinced-internal only, SSO-gated. No anonymous publish, no external
  contributor moderation.
- **Not a server-side package registry.** Source of truth is the git monorepo; "publish" =
  merge to `main`. No upload API, no artifact object store. `stark new`/`import` are
  **local scaffolding**, not publish.
- **Not a runtime/execution engine.** It distributes definitions; it does not run them.
- **Not a secrets store.** Secrets are referenced by name (`secretRef`); values never live
  in the catalog (§4.4).
- **Not a replacement for `stark-skills` tooling** (review dispatchers, automation fleet).
  It is the distribution layer; `stark-skills` remains an authoring source.

---

## 2. Constraints & assumptions

- **Audience:** Evinced engineers. Read/browse requires Evinced SSO (Google Workspace).
  Publish requires repo write + PR + CODEOWNERS review.
- **Hosting:** Private `GetEvinced/stark-marketplace`. Web registry on Evinced-controlled
  infra behind an identity-aware proxy (§10). All catalog-data surfaces (SPA, index,
  Claude dist tree, CLI fetch) sit behind SSO or private-repo auth — no anonymous origin.
- **Languages (workspace conventions):** Go for the engine + CLI (single static binary);
  TypeScript for the web SPA. **No Python.**
- **Determinism is a hard requirement, scoped to a pinned toolchain.** Generated output is
  a pure function of `(catalog source, adapter target versions)` given the pinned Go
  toolchain + serialization libraries (§7.6). CI fails on drift.
- **Runtimes are extensible** — a new runtime is a new adapter target + capability-matrix
  rows + goldens, not a schema change.

---

## 3. Architecture overview

```
                  ┌────────────────────────────────────────┐
                  │  catalog/  (hand-authored, canonical)    │
                  │   bundles → typed artifacts (md/yaml)    │
                  └───────────────────┬────────────────────┘
                                      │ load + validate (fail-closed)
                                      ▼
                  ┌────────────────────────────────────────┐
                  │              ENGINE (Go)                  │
                  │  loader · JSON-Schema validation          │
                  │  dependency DAG (presence + cycles)       │
                  │  adapter targets (per-runtime, versioned) │
                  │  cross-artifact aggregator (sentinels)    │
                  │  index builder (lean) + per-bundle detail │
                  │  CC marketplace generator                 │
                  │  hasher + CI-signed build manifest        │
                  └───────┬───────────────────┬─────────────┘
        deterministic     │                   │  derived metadata
        committed output  ▼                   ▼
   ┌──────────────────────────────┐   ┌──────────────────────────┐
   │ dist/claude/ (+ marketplace) │   │ index.json (lean) +       │
   │  COMMITTED — CC serves it     │   │ bundles/<name>.json detail│
   │ dist/codex, dist/gemini       │   │ COMMITTED, schema-versioned│
   │  NOT committed — built on     │   └────────────┬─────────────┘
   │  `stark install`              │                │
   └───────────────┬──────────────┘                │
       consumed by  │                               │ consumed by
   ┌────────────────┼────────────┐        ┌─────────┴────────────┐
   ▼                ▼            ▼         ▼                      ▼
┌───────────┐ ┌────────────┐ ┌─────────────┐          ┌──────────────────┐
│Claude Code│ │CLI search/ │ │CLI install  │          │ Web registry      │
│/plugin    │ │info/build/ │ │codex+gemini │          │ (SSO static SPA   │
│marketplace│ │validate/   │ │(adapt+merge │          │  over lean index +│
│add        │ │doctor      │ │ safely)     │          │  detail files)    │
└───────────┘ └────────────┘ └─────────────┘          └──────────────────┘
```

Components, dependency order (each ships independently):

| # | Component | Lang | Depends on |
|---|-----------|------|------------|
| ① | Schema + catalog layout + loader + validation | schema + Go | — |
| ② | Adapter targets (Claude first, then Codex/Gemini) + aggregator + determinism | Go | ① |
| ③ | Index builder (lean + detail) | Go | ① |
| ④ | CC marketplace generator | Go | ①② |
| ⑤ | CLI (`stark`: search/info/install/new/validate/build/doctor) | Go | ①②③④ |
| ⑥ | Web registry (SSO) | TS | ③ |

---

## 4. Canonical artifact format (Hybrid)

One file per artifact: `<type>/<name>.md` (MCP uses `<name>.yaml`, no body). Three layers:
frontmatter **superset**, **portable body**, optional **per-runtime overrides**.

### 4.1 Frontmatter (YAML)

```yaml
---
# ─ identity (required) ─
name: stark-review                 # slug ^[a-z0-9][a-z0-9-]{0,63}$ (no dots/slashes)
type: skill                        # skill | prompt | command | agent | mcp
description: Single-agent PR review with triage-selected domains.

# ─ catalog metadata (display fields inherit from bundle.yaml when unset) ─
version: 0.7.0                     # semver; required (see §11 for bump rules)
tags: [pr, review, multi-agent]
category: code-review
maturity: stable                   # experimental | beta | stable | deprecated
summary: One-line listing blurb.   # optional; falls back to description

# ─ runtime targeting (defaults to bundle.runtimes; may narrow, never widen) ─
runtimes: [claude, codex, gemini]

# ─ dependencies: presence + DAG only, NO version ranges (§7.3) ─
requires:
  - { type: skill, ref: stark-session }     # name (same bundle) or bundle/name

# ─ type-specific canonical fields (validated per §6; per-field fallback in §6.2) ─
argument-hint: "[PR_NUMBER] [--quick]"       # command-class field
model: opus                                   # mapped/dropped per runtime (§6.2)
disable-model-invocation: false

# ─ per-runtime overrides = AUTHOR INTENT ONLY (not emulation scaffolding, §6.1) ─
overrides:
  gemini: { model: gemini-2.5-pro }
---
```

### 4.2 Body — portable markdown + conditional fences

Body is runtime-neutral by default. Runtime-specific fragments use fences with a strict
grammar (regex, anchored at line start, case-insensitive runtime tokens):

```
FENCE_OPEN  = ^<!--\s*runtime:\s*(!?)([a-z0-9]+(?:\s*,\s*[a-z0-9]+)*)\s*-->\s*$
FENCE_CLOSE = ^<!--\s*/runtime\s*-->\s*$
```

- `<!-- runtime: claude, codex -->` includes the block only for those runtimes.
- `<!-- runtime: !claude -->` includes for **all targeted runtimes except** claude — the
  negative form so fences don't rot when a 4th runtime is added.
- No fence = included for every targeted runtime. Fences **may not nest**.
- **Error taxonomy:** unterminated fence, nested fence, unknown runtime token, runtime not
  in the artifact's targeted set, and a fence appearing inside a fenced code block are all
  validation **errors**. Fence stripping runs **before** any target-specific escaping
  (e.g. TOML string embedding) and an empty resulting section is an error.

### 4.3 Override merge semantics (deterministic, author-intent only)

1. Start from base frontmatter (after bundle inheritance, §5.2).
2. Deep-merge `overrides.<runtime>`: scalars replace, maps merge, **arrays replace
   wholesale** (no positional merge — predictability over convenience).
   - Wholesale replace is a known foot-gun for `tags`/`requires`/`mcp.args`. Validation
     emits a **warning** when an override array is not a superset-by-prefix of the base
     (likely an accidental drop). Documented at each array field.
3. Body = base body with non-matching fences stripped.
4. **Full-body replacement** (`overrides.<runtime>.body: |`) is a **lint error by
   default**. It is allowed only with an explicit `# diverged: <reason>` annotation;
   CI counts diverged artifacts and prints a **divergence budget** (e.g. "diverged 4 / 120
   = 3.3%") in PR output so copy-paste drift is visible, not buried.
5. **Emulation scaffolding is NOT done here** — see §6.1. Overrides express deliberate
   author divergence; the adapter owns the shape of emulated output.

Merge is pure and order-independent given §7.6 serialization, so output is byte-stable.

### 4.4 MCP payloads

MCP servers are configuration, no executable body:

```yaml
---
name: bigquery
type: mcp
description: Query BigQuery from the agent.
version: 1.2.0
runtimes: [claude, codex, gemini]
mcp:
  transport: stdio                 # stdio | http   (sse dropped: deprecated/uneven)
  command: stark-bq-mcp            # MUST be on the command allowlist (§7.4)
  args: ["--project", "${BQ_PROJECT}"]   # no inline-eval patterns (§7.4)
  env:
    BQ_PROJECT: { secretRef: bq-project-id }   # object form REQUIRED; string form illegal
  url: null                        # for http transport
---
```

- `env` values **must** be `{secretRef: <key>}` objects — free-form string values are a
  schema error (structural enforcement, not heuristic). `command`/`args`/`url` are scanned
  for inline credential patterns as defense-in-depth (§7.4).
- `command` must be on the allowlist; `args` may not contain inline-eval flags.

---

## 5. Catalog layout (bundle-first, typed within)

A **bundle** is the unit of **versioning** and **one CC plugin**. Install granularity is
**decoupled** from it (§9.1).

```
stark-marketplace/
  catalog/
    stark-review/
      bundle.yaml
      skills/stark-review.md
      commands/review.md
      agents/red-team.md
    stark-gh/
      bundle.yaml
      commands/pr-open.md
      mcp/gh.yaml
  dist/
    claude/                        # COMMITTED — CC serves this tree directly
      .claude-plugin/marketplace.json
      <bundle>/...
    # codex/ and gemini/ are NOT committed — built on `stark install` (§7.7)
  index.json                       # COMMITTED — lean search index (§7.5)
  bundles/<name>.json              # COMMITTED — per-bundle detail (§7.5)
  schema/                          # JSON Schemas (artifact/bundle/index)
  engine/                          # Go: loader, adapter, index, generator, CLI
  web/                             # TS: registry SPA
  docs/
  .gitattributes                   # catalog/** text eol=lf ; dist/** linguist-generated
```

### 5.1 What is committed (and why)

- **Committed:** `dist/claude/` (CC's `/plugin marketplace add` reads it from the repo),
  `index.json`, `bundles/*.json`. Marked `linguist-generated` so review collapses them.
- **Not committed:** `dist/codex/`, `dist/gemini/` — they have no in-repo consumer (the
  CLI adapts them at install). This removes most generated-file merge-conflict surface.
- **Drift check** (`stark build --check`) is a **required, non-bypassable** status; direct
  edits to committed generated paths require CODEOWNERS sign-off (§14). On conflict in
  generated files, the rule is **regenerate, never hand-merge** (`stark build --fix`).

### 5.2 `bundle.yaml`, inheritance, uniqueness

```yaml
name: stark-review
version: 0.7.0
description: Multi-agent PR review toolkit.
category: code-review
tags: [pr, review]
owner: { name: Evinced, email: engineering@evinced.com }
maturity: stable
runtimes: [claude, codex, gemini]
homepage: https://github.com/GetEvinced/stark-marketplace/tree/main/catalog/stark-review
```

- **Inheritable fields** (artifact inherits when unset): `category`, `tags`, `owner`,
  `maturity`, `runtimes`, `homepage`. Frontmatter overrides bundle; arrays **replace**, do
  not union (consistent with §4.3). `version` does **not** inherit.
- **`runtimes` single source:** artifact `runtimes` defaults to the bundle's and may only
  narrow; widening beyond the bundle is a validation error.
- **`name` uniqueness:** unique per bundle **across all types that share an output
  namespace in any targeted runtime** (e.g. on Codex/Gemini a skill, prompt, and command
  can land in the same dir). Validation computes per-runtime output paths and errors on any
  collision — preventing the cross-type file clash (red-team H14).

---

## 6. Artifact-type × runtime capability matrix

Per (type, runtime) support level: **native** (first-class), **emulated** (mapped onto the
nearest construct), **unsupported** (skipped, recorded). Corrected against current tool
docs (red-team Part B); exact keys/paths are pinned in `adapter/targets/<runtime>` and
covered by golden tests (§13).

| Canonical type | Claude Code | Codex (OpenAI) | Gemini CLI |
|----------------|-------------|----------------|------------|
| **skill** | native — `skills/<name>/SKILL.md` (frontmatter: name, description, disable-model-invocation, allowed-tools, user-invocable, effort, model*[*fork only]) | **native** — `.agents/skills/<name>/SKILL.md` (name+description required) | emulated — `GEMINI.md` sentinel block (+ optional `.gemini/commands/<name>.toml`); evaluate Gemini **Extensions** (§15) |
| **prompt** | native — `commands/<name>.md` | emulated — map to a **skill** (`~/.codex/prompts` is **deprecated**; `/prompts:<name>` if kept for parity) | native — `.gemini/commands/<name>.toml` (`prompt`+`description` only) |
| **command** (slash) | native — `commands/<name>.md` (frontmatter incl. `argument-hint`) | native — Codex **skill** invoked as `/skills`/`$name` (preferred over deprecated prompts) | native — `.gemini/commands/<name>.toml`; args via `{{args}}` |
| **agent** (subagent) | native — `agents/<name>.md` (name, description, tools, model, skills, effort, background, permissionMode) | emulated — Codex **skill** (no subagent primitive; optional `agents/openai.yaml` UI metadata) | emulated — `GEMINI.md` sentinel role block |
| **mcp** | native — `.mcp.json` / plugin `mcpServers` | native — `~/.codex/config.toml` `[mcp_servers.<name>]` *(verify exact key in adapter plan)* | native — `settings.json` `mcpServers.<name>` |

Rules:
- Validation **warns** on `emulated` targets, **errors** on `unsupported` targets.
- The matrix is versioned data, surfaced in the index for native/emulated badges.

### 6.1 Emulation is adapter-owned + carries a fidelity header

Emulated output's *shape* (arg-hints, reference blocks, skill-wrapping) is synthesized by
the adapter target from canonical fields — **not** authored via `overrides`. Every emulated
output gets a generated header: `EMULATED from <bundle>/<artifact> — derived shape; may not
auto-activate on this runtime; verify.` This keeps author-once intact and makes emulation
fidelity visible (red-team H1, ops-9).

### 6.2 Per-field capability fallback

Each canonical field declares per-runtime behavior so the adapter is fully specified:

| Field | Claude | Codex | Gemini |
|-------|--------|-------|--------|
| `model` | carry (skill: only with `context: fork`) | map to Codex model id or **drop+warn** | **drop+warn** (no field) |
| `argument-hint` | carry (command) | derive into skill usage text | render into `{{args}}` usage note |
| `disable-model-invocation` | carry | **drop+warn** | **drop+warn** |
| `allowed-tools`/`tools` | carry | best-effort skill metadata | **drop+warn** |

Behaviors: **carry** (emit as native field), **map** (translate value), **drop+warn**
(omit, count a warning), **error** (block). The full table lives in
`adapter/fieldmap.go`; the spec table is the contract for the common fields.

### 6.3 Cross-artifact aggregation (shared files)

When N artifacts target one shared file (`GEMINI.md`, `AGENTS.md`, a single
`config.toml`), the adapter aggregates deterministically:
- Each artifact's contribution is wrapped in stable sentinels:
  `<!-- stark:begin <bundle>/<name>@<digest> -->` … `<!-- stark:end <bundle>/<name> -->`.
- Sections are **sorted by `<bundle>/<name>`** (not map order), idempotent on rebuild.
- Install merges by sentinel (parse-replace), never blind-append (§9.2).

---

## 7. Engine (Go)

### 7.1 Pipeline

`build(catalog, targetVersions) → {dist, index, bundleDetail, manifest}`:
1. **Load** — walk `catalog/` (sorted), parse `bundle.yaml` + artifacts.
2. **Validate** — §7.4 (fail-closed).
3. **Resolve** — dependency DAG: presence + cycle detection + install ordering (§7.3).
4. **Adapt** — per (artifact, targeted runtime) via versioned targets; aggregate shared
   files (§6.3); deterministic formatting (§7.6).
5. **Index** — lean `index.json` + per-bundle `bundles/<name>.json` (§7.5).
6. **Generate marketplace** — `dist/claude/.claude-plugin/marketplace.json` (§8).
7. **Hash + sign** — content digests + CI-signed build manifest (§7.5).

### 7.2 Loader

Walks `catalog/` in sorted order, parses each `bundle.yaml` and artifact (frontmatter +
body), applies bundle inheritance (§5.2), and produces an in-memory model keyed by
`<bundle>/<type>/<name>`. The loader is pure (no network, no clock, no env reads beyond the
catalog) so it never introduces nondeterminism. Parse errors are collected and reported
together rather than failing on the first.

### 7.3 Dependency model (presence + DAG, no version SAT)

`requires[].ref` is `name` (same bundle) or `bundle/name`. Resolver checks the ref exists,
the graph is acyclic, and computes install order. **No semver ranges** — a single-version
monorepo can't use them and they only create spurious CI tripwires. An optional advisory
minimum may be recorded as metadata, never gated.

### 7.4 Validation (fail-closed, CI-gating) — security-first

- **Schema:** each artifact ↔ `schema/artifact.<type>.schema.json`; `bundle.yaml` ↔
  `schema/bundle.schema.json`.
- **Structural:** slug regex on `name`/`bundle.name`; output-path uniqueness (§5.2); fence
  grammar + error taxonomy (§4.2); override array foot-gun warning; `runtimes` narrowing.
- **Capability:** error on `unsupported`, warn on `emulated` (counted, surfaced).
- **MCP / code-execution (highest trust):**
  - `mcp.command` must be on a positive **command allowlist** (basenames; `stark-bq-mcp`,
    `node`, `uvx`, … opt-in). Everything else rejected.
  - `mcp.args` may not contain inline-eval patterns (`-e`, `-c`, `--eval`, unpinned `npx`).
  - `mcp.env` values must be `secretRef` objects; scan `args`/`url` for inline creds.
- **Secrets:** run gitleaks/trufflehog over the catalog in CI as defense-in-depth.
- **Path safety:** reject symlinks in `catalog/`; after path joins, canonicalize and assert
  containment within the intended root.
- **Body / content trust:** changes to skill/agent/command **bodies** and any `body:`
  override are flagged as **high-trust diffs** requiring a second CODEOWNERS reviewer;
  lint surfaces suspicious patterns (`curl … | sh`, reads of `.env`/`.private`, base64
  blobs, "ignore previous instructions"). `agent.tools` validated against an allowlist and
  surfaced in the index.
- **Integrity / drift:** committed `dist/claude/` + `index.json` + `bundles/*.json` must
  equal a fresh build; `stark build --check` is a required status.

### 7.5 Index + provenance (lean, scalable, signed)

- **Lean `index.json`** — only what search needs: `{name, type, bundle, tags, category,
  maturity, support badges, version, digest}` + `schemaVersion`. Keeps first-paint small.
- **Per-bundle `bundles/<name>.json`** — full detail (artifacts, deps closure, per-runtime
  output paths, fidelity notes), fetched on demand by SPA/CLI.
- **Backward-compat by convention:** additive fields only within a `schemaVersion`;
  consumers **ignore unknown fields** rather than hard-fail. `schemaVersion` bumps are
  genuine breaks and supported **N-1** so older `stark` binaries keep working.
- **Provenance:** a CI-signed build manifest (GitHub OIDC → sigstore/cosign keyless, or a
  CI-only KMS key developers can't write) records target versions + digests. This — plus
  protected `main` + commit SHA — is the trust root. Self-computed digests are explicitly
  only a **consistency/anti-drift** mechanism, not provenance (red-team C1).

### 7.6 Determinism contract (the load-bearing claim)

- All emitted maps **sorted by key** (frontmatter + JSON); ordered encoders only; never
  rely on source key order or Go map iteration.
- All directory walks + collection iterations explicitly sorted.
- **LF** enforced on read and write; **`/`** path separators in all generated content.
- **Pinned Go toolchain** (`go.mod toolchain` + CI) and pinned serialization libs;
  `.gitattributes` forces `catalog/** text eol=lf`. A toolchain bump is treated like an
  adapter bump (§7.7).
- **Determinism test (§13):** build twice → identical; and a test that **reorders source
  frontmatter keys** and asserts byte-identical output.

### 7.7 Adapter target versioning + rollout

- Each runtime target is **independently versioned** (`adapter/targets/gemini@N`) so a
  Gemini format fix churns only Gemini output, not all three.
- Target versions appear in `index`/detail; `stark install` warns "installed with target
  X, current Y — re-install recommended."
- An adapter-version bump is its **own PR type** with a generated changelog of which
  outputs changed; large regenerated diffs are expected and reviewed as a format change,
  not buried in a content PR. Provenance/install digests include target versions so an
  adapter bump correctly invalidates installed bytes (red-team H11, ops-6/7).

---

## 8. CC marketplace generator (④)

Emits `dist/claude/.claude-plugin/marketplace.json` in the **native** Claude Code format:
- **Root:** `owner` (name/email).
- **Per `plugins[]` entry:** `author` (not `owner`), `source` (string or object —
  `github`/`url`/`git-subdir`), `version`, `description`, `category`, `tags`, `strict`.
- One entry per bundle; `source` points at the bundle's committed `dist/claude/<bundle>/`.
Because the catalog layout is isomorphic to CC's plugin layout, generation is a projection
of the index + a copy of each bundle's Claude tree. Users:
`/plugin marketplace add GetEvinced/stark-marketplace` → `/plugin install <bundle>`. No
server. (Corrects red-team Part B: `author` vs `owner`.)

---

## 9. CLI (`stark`)

Single Go binary. Reads committed `index.json` / `bundles/*.json` (local checkout or
authenticated fetch, §9.5). `new`/`import` are local scaffolding only.

### 9.1 Commands & install granularity

| Command | Purpose |
|---------|---------|
| `stark search <q> [--type --tag --runtime --maturity]` | Query the lean index. |
| `stark info <bundle[/artifact]>` | Metadata, support matrix, **dependency closure**, install preview. |
| `stark install <bundle\|bundle/artifact> --runtime <r> [--dest <p>] [--plan] [--force]` | Install. Bundle-install on runtime R = the **subset of artifacts targeting R**, skipping + reporting the rest. Resolves deps in DAG order. |
| `stark new <type> --bundle <b>` | Scaffold a canonical artifact (local). |
| `stark import --from <path>` | Scaffold bundles from a `stark-skills` checkout (local). |
| `stark validate [path]` | Full §7.4 validation. |
| `stark build [--check\|--fix]` | Build dist+index; `--check` = drift gate (CI); `--fix` regenerates. |
| `stark doctor` | Verify installed manifests still match the index; sentinel blocks intact; report broken/emulated installs. |
| `stark version` / `stark self-update` | Version + supported `schemaVersion` range; update binary. |

### 9.2 Safe install into shared config files

- MCP install **merges by key**: parse `config.toml`/`settings.json`, insert/replace only
  the managed `[mcp_servers.<name>]` / `mcpServers.<name>` subtree, **preserving comments,
  ordering, and all other content** (comment-preserving TOML editor for Codex). Atomic
  temp+rename. Collision with an **unmanaged** block → refuse without `--force`.
- Shared `AGENTS.md`/`GEMINI.md` contributions are written as **sentinel blocks** (§6.3),
  parse-replace by sentinel — never blind-append. Refuse to overwrite unsentineled user
  content inside the managed region.
- An **advisory file lock** is held on each shared file during mutation.
- The install manifest records the exact **key paths / sentinels** written, so `--remove`
  excises precisely what was added.

### 9.3 Consent for code-executing classes

Installing `mcp` or `agent` artifacts is **never silent**: `stark install` prints the exact
command+args (MCP) / tool grants (agent) and the full resolved **dependency closure**
(highlighting any `mcp`/`agent` pulled in transitively) and requires explicit confirmation.
`--plan` shows the same without writing.

### 9.4 Atomicity & rollback

Stage all adapted output to a temp area; **pre-validate all digests**; then commit with
atomic renames. A **write-ahead journal** records each mutation before it happens, fsynced,
so a crashed/partial install is recoverable by `stark install --remove`/`--repair`.

### 9.5 Auth for fetch

When not using a local checkout, `stark` fetches index/detail via the **authenticated
GitHub API against the private repo** (or a gated endpoint) — never an anonymous raw URL.
SSO/repo-access is required for all catalog data (red-team sec-6).

### 9.6 Local observability (no network telemetry in v1)

`stark install` prints a clear success/failure summary and writes a per-run log;
`stark doctor` audits installed state. Emulated outputs carry the §6.1 fidelity header.
Aggregate analytics deferred (§15), local signals are not.

### 9.7 Distribution / bootstrap

`stark` ships as **signed release binaries** per merge (GitHub Releases / internal artifact
store); `go install …/engine/cmd/stark@latest` documented as fallback. Every command
asserts its supported `index.schemaVersion` range and tells the user to update when out of
range.

### 9.8 Error/exit-code contract

| Exit | Meaning |
|------|---------|
| 0 | success |
| 1 | validation error |
| 2 | drift (`build --check` mismatch) |
| 3 | digest/integrity mismatch |
| 4 | install conflict (use `--force`) |
| 5 | unsupported `schemaVersion` (update `stark`) |
| 6 | user declined consent |

`--json` emits machine-readable results for every command; warnings vs errors map to
stderr + exit code.

---

## 10. Web registry (⑥, SSO-gated)

- **Shape:** static SPA (TypeScript, Vite) over the **lean index** (search) + per-bundle
  **detail files** (on demand). No app server for data.
- **Auth:** behind an identity-aware proxy enforcing Evinced Google Workspace SSO (GCP
  Cloud Run + IAP or the Evinced-standard gated-static pattern). The proxy gates **all**
  data files (index, detail, any served Claude tree), not just HTML routes. No app-level
  user store.
- **Deploy:** CI builds SPA + index on merge and publishes as **one atomic,
  content-hashed unit** (hashed long-cache assets; the index pointer is cache-busted with
  the SPA build hash). On `schemaVersion`/version skew the SPA **degrades gracefully**
  ("registry updating / unsupported index — here's the GitHub source"), never blank-fails.
- **Features:** faceted search (type/tag/category/runtime/maturity), bundle+artifact
  detail, per-surface install instructions, native/emulated badges, dependency graph,
  deep links to source.

---

## 11. Versioning, immutability, provenance

- **Semver per bundle and artifact.**
- **Version-bump gate** hashes a **normalized canonical source form** (sorted frontmatter,
  normalized whitespace/LF) so cosmetic edits and **display-metadata-only** changes
  (`description`/`tags`/`summary`) do **not** force a bump; only body + type-specific
  canonical fields do. Empty-previous-index (first commit) = no check.
- **Install/provenance digest** is computed over the **generated per-runtime output and
  includes the adapter target version**, so an adapter bump correctly invalidates installed
  bytes (consumers can detect "re-install recommended").
- **Immutability** holds under **protected, linear `main`** (no force-push) + the
  CI-signed manifest; installs may **pin the commit SHA**. Documented as conditional on
  branch protection, not a cryptographic guarantee absent the signed manifest.
- **Deprecation:** `maturity: deprecated` stays installable, flagged, excluded from default
  search.

---

## 12. Migration from `stark-skills`

- Existing `skill/<name>/SKILL.md` bodies are already portable; migration is mostly
  **mechanical**: move into `catalog/<bundle>/skills/<name>.md`, extend frontmatter
  (version/tags/category/maturity/runtimes), group related artifacts into a bundle.
- `plugins/stark-gh` maps directly to a `stark-gh` bundle (commands + mcp).
- `stark import --from <stark-skills path>` (local scaffolding) generates bundles and
  reports what needs human metadata. Lands **bundle-by-bundle** (deployable slices), not
  big-bang.
- `stark-skills` remains the authoring/automation home; only shareable artifacts move.

---

## 13. Testing strategy

- **Golden-file tests** per runtime target (canonical → native output, byte-exact);
  format drift updates only that target + goldens.
- **Determinism tests:** build-twice identity **and** source-key-reorder identity (§7.6).
- **Schema conformance** for all examples + catalog artifacts.
- **Drift test (CI):** `stark build --check`.
- **Property tests:** override merge (idempotence/precedence/array-replace), fence
  stripping (incl. `!` form + error taxonomy), cross-artifact aggregation idempotence.
- **Dependency resolver:** cycles, missing refs, ordering.
- **Validation:** positive + negative fixtures per rule — esp. command allowlist,
  inline-eval rejection, `secretRef` enforcement, path-traversal, slug regex.
- **Install integration (all 3 runtimes, temp dirs):** install → verify → `--remove` →
  verify clean; shared-file merge preserves pre-existing content + comments; collision
  refusal; crash-mid-install → `--repair`; idempotent re-install.
- **Web smoke:** SPA builds against a fixture index; search + detail render; graceful
  degrade on schema skew.

---

## 14. CI / ops

- **PR gates (required, non-bypassable):** `stark validate`, `stark build --check` (drift),
  golden + determinism + integration tests, web build, gitleaks. Capability/array
  **warnings** reported but non-blocking; **errors** block.
- **CODEOWNERS:** `catalog/**` + `schema/**` reviewed; `**/mcp/**` and high-trust **body**
  diffs require a second reviewer; `engine/**`, committed `dist/claude/**`, `index.json`,
  `bundles/**` require maintainer review. Aryeh on sensitive paths.
- **Branch protection:** protected linear `main`, no force-push, drift check required, no
  admin bypass on this repo.
- **Merge → deploy:** committed Claude dist + index already match (drift gate); web deploy
  publishes the atomic content-hashed SPA + index; signed build manifest attached.

---

## 15. Open questions / risks

1. **Codex MCP key** (`[mcp_servers.<name>]`) and Codex skill dir precedence — confirm
   against current Codex docs as the first adapter-plan task; goldens pin the result.
2. **Gemini Extensions** vs ad-hoc `GEMINI.md` blocks as the skill/agent emulation target —
   evaluate before locking Gemini goldens (Extensions may be more faithful + uninstallable
   cleanly).
3. **Emulation fidelity bar** per (type, runtime) — define acceptance criteria; surface
   limits in `info`/web; the fidelity header (§6.1) is the stopgap.
4. **Command allowlist governance** — who approves additions; start minimal.
5. **Web hosting pattern** — pick the Evinced-standard gated-static pattern; no ad-hoc
   provisioning.
6. **Aggregate telemetry** — deferred to post-v1; must stay internal-only if added.
7. **`stark` name collision** with any existing internal binary — confirm before
   distributing.

---

## 16. Build sequencing (feeds the plans)

1. **Foundation** — schema + catalog layout + loader + validation + `bundle.yaml` +
   determinism scaffolding (`.gitattributes`, pinned toolchain). Deliverable:
   `stark validate` green on a seed catalog; slug/path/fence/secret rules enforced.
2. **Engine core (Claude target)** — adapter (Claude) + lean index + per-bundle detail +
   determinism contract + drift. Deliverable: `stark build` emits committed Claude tree +
   index; golden + determinism + drift tests.
3. **Multi-runtime** — Codex + Gemini targets (corrected matrix), per-field fallback,
   cross-artifact aggregation, per-target versioning. Deliverable: 3-runtime output +
   goldens + emulation headers/badges (codex/gemini dist uncommitted).
4. **CC marketplace** — generator (`author`/`source` correctness) + native install loop.
   Deliverable: `/plugin marketplace add` end-to-end.
5. **CLI** — search/info/install (safe-merge, consent, atomic+journal, doctor) + exit-code
   contract + distribution/self-update. Deliverable: safe parity install for Codex/Gemini.
6. **Web registry** — SSO-gated atomic static SPA over index + detail. Deliverable:
   browse/search behind Evinced SSO with graceful degrade.
7. **Migration** — `stark import` + move `stark-skills` artifacts bundle-by-bundle.
8. **Security hardening pass** — CI-signed build manifest (sigstore/OIDC), command-allowlist
   governance, gitleaks, branch protection + CODEOWNERS wiring verified end-to-end.

Each step is an independently shippable slice with its own implementation plan.
```
