# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`stark-marketplace` is the **canonical, multi-runtime marketplace** for stark bundles. The engine renders each bundle into per-runtime trees (`dist/claude/`, `dist/codex/`, `dist/gemini/`) plus a signed `index.json` / `bundles/*.json` web registry. The repo doubles as a native **Claude Code marketplace** — `.claude-plugin/marketplace.json` at the repo root IS the manifest CC reads when you `/plugin marketplace add 21-Stark-AI/stark-marketplace`.

**Source of truth = stark-skills.** The catalog's `skills/` + `commands/` are **generated** from a stark-skills checkout by `stark sync` (driven by each `bundle.yaml`'s `skills:`/`commands:` membership manifest); do NOT hand-edit them. What IS curated in this repo: each `bundle.yaml` (metadata + membership), and `mcp/` artifacts (stark-skills defines no MCP servers). See "Generation pipeline" below.

**Plugins are self-contained.** `stark build` vendors the stark-skills immutable assets (`tools/`, `prompts/`, `standards/`, `scripts/`, `config.json`, `forge_heuristics.json`) — from the committed `vendor/stark-skills/` snapshot — into every `dist/claude/<bundle>/`, and emits a per-bundle `.claude-plugin/plugin.json`. So `/plugin install <bundle>` works with **no install.sh** on the target machine. Skills resolve paths via `${CLAUDE_PLUGIN_ROOT}`. Runtime prereq: **Node ≥ 22.6** (`node --experimental-strip-types`).

**Per-bundle plugin assets.** A bundle generated from a stark-skills `plugins/<bundle>/` plugin (e.g. `stark-gh`) often has **plugin-specific** tools + its own `config.json` that the shared snapshot does NOT carry — the shared `tools/` is stark-skills' top-level dir, not `plugins/<bundle>/tools/`. `stark sync` captures these into `vendor/plugins/<bundle>/` (`importer.PluginVendorSnapshot`: `tools/**` `.ts` + `config.json` + `package.json`), and `stark build` layers them into that bundle's `dist/claude/<bundle>/` **after** the shared snapshot, so the plugin's own files win (e.g. stark-gh's `gh_*.ts` tools + its `{draft}` `config.json` override the global `config.json`). Without this, commands like `/stark-gh:cleanup` crash with `MODULE_NOT_FOUND` because their `${CLAUDE_PLUGIN_ROOT}/tools/gh_cleanup.ts` was never vendored.

## Layout

- `catalog/<bundle>/` — `bundle.yaml` + `mcp/` are **curated** (edit here); `skills/` + `commands/` are **generated** by `stark sync` from stark-skills (do not hand-edit).
- `vendor/stark-skills/` — **generated** normalized snapshot of stark-skills' immutable assets (tools/prompts/standards/scripts/config), refreshed by `stark sync`, vendored into each plugin by `stark build`. Committed, `linguist-generated`.
- `vendor/plugins/<bundle>/` — **generated** per-bundle plugin asset snapshot (plugin-specific `tools/**` + the plugin's own `config.json`/`package.json`), refreshed by `stark sync` from stark-skills `plugins/<bundle>/`, layered by `stark build` into that bundle's dist tree (winning over `vendor/stark-skills/`). Only exists for bundles backed by a stark-skills plugin (currently `stark-gh`). Committed, `linguist-generated`.
- `engine/` — Go CLI (`cmd/stark`) + libraries (`internal/{adapter,build,validate,importer,install,...}`). Per-runtime adapters live under `internal/adapter/{claude,codex,gemini}`. The generator lives in `cmd/stark/sync.go` + `internal/importer/{importer,vendor,serialize}.go`.
- `dist/claude/` — **committed** rendered tree (CC consumes it directly). `dist/codex/` and `dist/gemini/` are **NOT committed** — built on `stark install`.
- `bundles/*.json`, `index.json` — committed, signed web-registry payloads. Marked `linguist-generated`.
- `schema/` — JSON Schemas (`bundle`, `artifact.{skill,command,agent,prompt,mcp}`). Fail-closed in CI.
- `server/` — small Go static origin for the web registry (Cloud Run behind the ev-infra-group platform LB at `marketplace.21stark.com`). No auth in-process.
- `web/` — strict-TS Vite/React SPA shell over `index.json` (HashRouter, relative fetches).
- `.claude-plugin/marketplace.json` — repo-root CC marketplace manifest (must stay at repo root; CC resolves plugin `source` paths relative to it).
- `docs/` — `native-install-loop.md`, `web-hosting.md`, `SECURITY.md`. Read these before changing install/deploy/governance behavior.

## Common commands

Engine (run from `engine/`):
- `go test ./... -count=1` — unit + golden + determinism + integration
- `go vet ./...`
- `go run ./cmd/stark sync --from <stark-skills> ../catalog` — **regenerate** catalog skills/commands + `vendor/` from the source repo (`--check` = drift gate)
- `go run ./cmd/stark validate ../catalog` — fail-closed schema + cross-ref
- `go run ./cmd/stark build --check ../catalog` — **drift gate**; rebuild must equal committed `dist/claude/` + registry (vendors `vendor/stark-skills` into each bundle)
- `go run ./cmd/stark check-bumps ../catalog` — version-bump immutability (bump the **bundle** version to publish a content change)
- `go run ./cmd/stark lint ../catalog` — body suspicious-pattern scan (non-blocking)
- `go run ./cmd/stark install <bundle> --runtime claude|codex|gemini` — render into local config

Web (run from `web/`):
- `npm run dev` / `npm run build` (does `tsc --noEmit && vite build`) / `npm run test` / `npm run lint` / `npm run typecheck`

Server (run from `server/`): `go test ./...`, `go run . ` (env: `WEBROOT`, `PORT`).

CI mirrors these exactly (`.github/workflows/ci.yml`): validate → drift `build --check` → check-bumps → lint → tests → gitleaks. All blocking except `lint`.

## Architecture — the bits that need multiple files to understand

**Source → adapter → output.** `internal/load` reads `catalog/`; `internal/adapter/{claude,codex,gemini}` each implement a `Target` that renders a `model.Bundle` into a runtime-specific file tree. `internal/build` drives a full render; `internal/install` consumes a per-runtime render to write into a user's CC/codex/gemini config. `cmd/stark` wires these together. The `catalogAdapter` in `engine/cmd/stark/` is the production `installplan.Adapter` — renders **one artifact at a time** in a single-artifact sub-bundle so a multi-MCP bundle doesn't collide on `config.toml`.

**Generation pipeline (stark-skills → catalog → dist).** `stark sync` (`cmd/stark/sync.go`) reads a stark-skills checkout and, per each `bundle.yaml`'s `skills:`/`commands:` membership, writes `catalog/<bundle>/{skills,commands}` via `importer.ImportForGenerator` + `ArtifactFiles` (preserving the curated `bundle.yaml` + `mcp/`), refreshes the shared `vendor/stark-skills/` via `importer.VendorSnapshot`, and refreshes each bundle's `vendor/plugins/<bundle>/` via `importer.PluginVendorSnapshot`. Generated artifacts inherit their bundle's `version` + `runtimes`. `stark build` then vendors `vendor/stark-skills/` into each `dist/claude/<bundle>/` (`build.Options.AssetsSource`, defaulting to `<repo>/vendor/stark-skills`) and layers `vendor/plugins/<bundle>/` on top for the owning bundle (`build.Options.PluginAssetsRoot`, defaulting to `<repo>/vendor/plugins`). Two drift gates: `sync --check` (catalog/vendor vs a fresh generation — cross-repo) and `build --check` (dist vs catalog). CI wires stark-skills → here automatically (auto-publish PR on stark-skills `main`).

**Determinism is load-bearing.** `build --check` is the drift gate. Any change that re-renders bytes — adapter output, schema, canonicalization (`internal/canonjson`), aggregate index (`internal/aggregate`), digest (`internal/digest`) — will fail CI unless you also commit the regenerated `dist/claude/`, `index.json`, `bundles/*.json`. Workflow: edit catalog → `go run ./cmd/stark build ../catalog` → commit the diff.

**Spec §5.1 commit rule.** Only `dist/claude/` is committed (CC needs it at-rest). `dist/codex/` and `dist/gemini/` are produced by `stark install` on the user's machine — never commit them.

**Version-bump immutability** (`internal/bumps`): an artifact at a given version is content-locked. Bump the version when content changes; `check-bumps` blocks otherwise.

**Web registry signing.** `sign-manifest.yml` workflow signs `index.json`. The web SPA fetches `./index.json` and `./bundles/<name>.json` relative to the document; the static origin serves no-cache for these and immutable long-cache for hashed assets.

## Conventions

- **Go 1.24**, module `github.com/21-Stark-AI/stark-marketplace/engine`. Standard layout (`cmd/`, `internal/`), small packages.
- **TypeScript strict** in `web/`. ESM, narrow types.
- **Never hand-edit `dist/`, `index.json`, or `bundles/*.json`.** Regenerate via `stark build`.
- **CODEOWNERS** gates schema, adapters, governance, signing, deploy — respect the gates; don't try to route around them.
- See `docs/native-install-loop.md` for the CC install loop and `docs/SECURITY.md` for the threat model and signing policy.
