# stark-marketplace

Canonical, multi-runtime marketplace for **stark** bundles. One source of truth (`catalog/`) renders into per-runtime trees for **Claude Code**, **Codex**, and **Gemini CLI**, plus a signed web registry served behind SSO at [marketplace.evinced.rocks](https://marketplace.evinced.rocks).

The repo is also a **native Claude Code marketplace**: `.claude-plugin/marketplace.json` at the repo root is the manifest CC reads directly — no custom client.

## Bundles

| Bundle | Purpose |
| --- | --- |
| `stark-constitution` | Project setup & session priming (spec-kit `constitution` phase). |
| `stark-plan` | Plan-time guidance (spec-kit `plan` phase). |
| `stark-analyze` | Multi-domain review + adversarial red-team of designs/plans/PRs. |
| `stark-implement` | Implementation-time guidance (spec-kit `implement` phase). |
| `stark-gh` | GitHub workflow commands + MCP. |
| `stark-ops` | Ops/runtime utilities. |

## Install (Claude Code)

```
/plugin marketplace add 21-Stark-AI/stark-marketplace
/plugin install stark-gh@stark-marketplace
```

Each plugin is **self-contained** — its supporting tool scripts, prompts, and config are vendored into the bundle, so `/plugin install` works with **no `install.sh`** and no stark-skills checkout on your machine. Only prerequisite: **Node ≥ 22.6** (skills run `node --experimental-strip-types`; `stark doctor` checks it).

Other runtimes go through the engine CLI:

```bash
cd engine
go run ./cmd/stark install stark-gh --runtime codex   # or gemini
```

See [`docs/native-install-loop.md`](docs/native-install-loop.md) for the full install loop.

## Develop

**Source of truth is the [stark-skills](https://github.com/21-Stark-AI/stark-skills) repo**, not this one. The catalog's `skills/` + `commands/` and the `vendor/stark-skills/` snapshot are **generated** from a stark-skills checkout — never hand-edit them, `dist/`, `index.json`, or `bundles/*.json`. What you DO edit here: each `catalog/<bundle>/bundle.yaml` (metadata + the `skills:`/`commands:` membership manifest) and curated `catalog/<bundle>/mcp/` artifacts.

Standard loop:

```bash
cd engine
# 1. regenerate catalog skills/commands + vendor/ from a stark-skills checkout
go run ./cmd/stark sync --from ../../stark-skills ../catalog
# 2. render dist/ (vendors vendor/stark-skills into each bundle) + manifests
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

Static origin (`server/`) is the Cloud Run image fronting the registry behind IAP.

## Architecture, in one paragraph

`internal/load` parses `catalog/` into a `model.Catalog`. Per-runtime adapters in `internal/adapter/{claude,codex,gemini}` render bundles into runtime-specific file trees. `internal/build` drives a full render; `internal/install` consumes one per-runtime render and writes it into a user's config. `cmd/stark` wires these. Determinism is load-bearing: `build --check` is the drift gate and `check-bumps` enforces version-bump immutability. Only `dist/claude/` is committed (CC consumes it at-rest); `dist/codex/` and `dist/gemini/` are produced by `stark install` on the user's machine.

More detail in [`CLAUDE.md`](CLAUDE.md).

## Trust & governance

This distributes code that runs inside developer agents and, for `mcp/` entries, spawns commands on developer machines. Integrity rests on:

1. Protected, linear `main` (no force-push, no bypass).
2. CI-signed build manifest (GitHub OIDC → sigstore/cosign keyless, signer `repo:21-Stark-AI/stark-marketplace@refs/heads/main`).
3. Commit SHA, which the manifest binds digests to.

`stark verify-manifest` checks all three. Self-computed digests alone are only an anti-drift signal.

MCP `command` values and `agent.tools` must be on positive allowlists (`engine/internal/validate/{allowlist,toolsallow}.go`). Adding an entry requires a CODEOWNERS-gated PR with maintainer + `@aryeh-evinced` approval.

Full threat model and controls: [`docs/SECURITY.md`](docs/SECURITY.md).

## Documentation map

- [`CLAUDE.md`](CLAUDE.md) — orientation for Claude Code / agentic contributors.
- [`AGENTS.md`](AGENTS.md) — same orientation for Codex / Gemini.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — how to add or change a bundle/artifact.
- [`docs/native-install-loop.md`](docs/native-install-loop.md) — end-to-end install via CC native marketplace.
- [`docs/SECURITY.md`](docs/SECURITY.md) — trust model, signing, allowlist process, branch protection.
- [`docs/web-hosting.md`](docs/web-hosting.md) — Cloud Run + IAP wiring for `marketplace.evinced.rocks`.
- [`web/README.md`](web/README.md) — SPA-specific dev notes.

## License & ownership

Internal Evinced project. See [`CODEOWNERS`](CODEOWNERS) for review gates.
