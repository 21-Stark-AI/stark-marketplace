# Bifröst

The bridge between the realms. Canonical, multi-runtime marketplace for **stark** bundles — one source of truth (`catalog/`) renders into per-runtime trees for **Claude Code**, **Codex**, and **Gemini CLI**, plus a signed web registry served at [marketplace.21stark.com](https://marketplace.21stark.com). GitHub slug: `21StarkCom/bifrost`.

The repo is also a native marketplace for both hosts. Claude Code reads `.claude-plugin/marketplace.json`; Codex prefers `.agents/plugins/marketplace.json`. Each manifest points at a separate generated package tree, so Codex compatibility cannot alter Claude's installed workflows.

## Bundles

| Bundle | Purpose |
| --- | --- |
| `stark-constitution` | Project setup & session priming (spec-kit `constitution` phase). |
| `stark-plan` | Plan-time guidance (spec-kit `plan` phase). |
| `stark-analyze` | Multi-domain review + adversarial red-team of designs/plans/PRs. |
| `stark-implement` | Implementation-time guidance (spec-kit `implement` phase). |
| `stark-ops` | Ops/runtime utilities. |
| `stark-design` | Design-token architecture — three-tier tokens, OKLCH scales, DTCG/Style Dictionary, theming, WCAG gates. |
| `stark-write` | Long-form writing craft — voice profile, story editing, adversarial sharpening, multi-model jury. |

## Install (Claude Code)

```
/plugin marketplace add 21StarkCom/bifrost
/plugin install stark-ops@bifrost
```

Each plugin is **self-contained** — its supporting tool scripts, prompts, config, and standard per-skill `references/`, `scripts/`, and `assets/` are vendored into the bundle, so `/plugin install` works with **no `install.sh`** and no stark-skills checkout on your machine. Only prerequisite: **Node ≥ 22.6** (skills run `node --experimental-strip-types`; `stark doctor` checks it).

## Install (Codex)

```bash
codex plugin marketplace add 21StarkCom/bifrost
codex plugin add stark-ops@bifrost
```

Start a new thread after installing or updating, then invoke housekeeping with
`$stark-housekeeping --dry-run` (or select it through `/skills`). Codex installs
native skills from `dist/codex-plugins/`; it does not migrate the Claude command
files.

Direct/project-local Codex installs still go through the engine CLI:

```bash
cd engine
go run ./cmd/stark install stark-ops --runtime codex   # standalone Codex
```

See [`docs/native-install-loop.md`](docs/native-install-loop.md) for the full install loop.

## Develop

**Source of truth is the [stark-skills](https://github.com/21-Stark-AI/stark-skills) repo**, not this one. The catalog's `skills/` + `commands/`, shared vendor snapshot, and Codex-only runtime-overlay snapshot are **generated** from a stark-skills checkout — never hand-edit them, either generated runtime package tree, `index.json`, or `bundles/*.json`. What you DO edit here: each `catalog/<bundle>/bundle.yaml` (metadata + the `skills:`/`commands:` membership manifest) and curated `catalog/<bundle>/mcp/` artifacts.

Standard loop:

```bash
cd engine
# 1. regenerate catalog skills/commands + vendor/ from a stark-skills checkout
go run ./cmd/stark sync --from ../../stark-skills ../catalog
# 2. render dist/claude + dist/codex-plugins and both native manifests
go run ./cmd/stark build ../catalog
# 3. gates — all must be clean before pushing
go run ./cmd/stark validate ../catalog
go run ./cmd/stark sync --from ../../stark-skills --check ../catalog   # catalog/vendor drift
go run ./cmd/stark build --check ../catalog                            # dist drift
go run ./cmd/stark check-bumps ../catalog                              # bump bundle version on content change
go test ./... -count=1
```

To publish a skill change: edit it in **stark-skills**, bump the affected bundle's `version` in its `bundle.yaml` here, then run the loop. CI does this automatically — a push to stark-skills `main` regenerates and opens a PR here (`.github/workflows/marketplace-sync.yml` in stark-skills). This repo's `.github/workflows/ci.yml` then gates the PR: `validate → build --check → check-bumps → lint → tests → gitleaks`. All blocking except `lint`.

Web SPA (`web/`):

```bash
cd web
npm install
npm run dev        # local
npm run build      # tsc --noEmit && vite build
npm test
```

Static origin (`server/`) is the Cloud Run image fronting the registry behind the
`ev-infra-group` platform load balancer.

## Architecture, in one paragraph

`internal/load` parses `catalog/` into a `model.Catalog`. Per-runtime adapters in `internal/adapter/{claude,codex,gemini}` render bundles into runtime-specific file trees. `internal/build` independently generates committed Claude packages in `dist/claude/` and native marketplace Codex packages in `dist/codex-plugins/`; `internal/install` still produces standalone/project-local runtime installs. `cmd/stark` wires these. Determinism is load-bearing: `build --check` is the drift gate and `check-bumps` enforces canonical version-bump immutability. `dist/codex/` and `dist/gemini/` remain ignored standalone install outputs.

More detail in [`CLAUDE.md`](CLAUDE.md).

## Trust & governance

This distributes code that runs inside developer agents and, for `mcp/` entries, spawns commands on developer machines. Integrity rests on:

1. Protected, linear `main` (no force-push, no bypass).
2. CI-signed build manifest (GitHub OIDC → sigstore/cosign keyless, signer `repo:21StarkCom/bifrost@refs/heads/main`).
3. Commit SHA, which the manifest binds digests to.

`stark verify-manifest` checks all three. Self-computed digests alone are only an anti-drift signal.

MCP `command` values and `agent.tools` must be on positive allowlists (`engine/internal/validate/{allowlist,toolsallow}.go`). Adding an entry requires a CODEOWNERS-gated PR with maintainer + `@aryeh-stark` approval.

Full threat model and controls: [`docs/SECURITY.md`](docs/SECURITY.md).

## Documentation map

- [`CLAUDE.md`](CLAUDE.md) — orientation for Claude Code / agentic contributors.
- [`AGENTS.md`](AGENTS.md) — same orientation for Codex / Gemini.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — how to add or change a bundle/artifact.
- [`docs/native-install-loop.md`](docs/native-install-loop.md) — end-to-end install via CC native marketplace.
- [`docs/SECURITY.md`](docs/SECURITY.md) — trust model, signing, allowlist process, branch protection.
- [`docs/web-hosting.md`](docs/web-hosting.md) — Cloud Run + LB wiring for `marketplace.21stark.com`.
- [`web/README.md`](web/README.md) — SPA-specific dev notes.

## License & ownership

Internal 21 Stark AI project. See [`CODEOWNERS`](CODEOWNERS) for review gates.
