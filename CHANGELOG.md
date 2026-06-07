# Changelog

All notable changes to `stark-marketplace`. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [SemVer](https://semver.org/spec/v2.0.0.html). Bumping `VERSION` on `main` triggers a tag + signed release.

## [Unreleased]

## [0.1.4] — 2026-06-07

First release covering all items in `docs/plans/prod-ready-followup-2026-06-07.md`.

### Added
- `server/`: baseline security headers (HSTS, CSP, Permissions-Policy,
  X-Frame-Options, tightened Referrer-Policy) on every response. Tests
  in `server/main_test.go` assert headers land on healthz, asset, data,
  SPA fallback, 405, and HEAD responses.
- New `stark allowlist` subcommand that prints the canonical Markdown
  view of `commandAllowlist` + `agentToolAllowlist`. `--check <path>`
  drift-gates a committed copy.
- `docs/allowlist.md`: generated from the two `engine/internal/validate`
  allowlists; CI fails closed if it drifts from the source.
- `docs/operations/rollback.md`: runbook covering Cloud Run revision
  rollback, bundle yank policy (content-locked + advisory), and
  signed-release revocation (cosign has no native revoke).
- Diagnostic step in `sign-manifest.yml` capturing git state after
  `stark build` to investigate the Linux-only "19 files deleted" mystery
  that forces `goreleaser --skip=validate` today (workaround unchanged).

### Changed
- CI gates web with `npm typecheck`, `npm run lint`, and `npm test`
  before `npm run build` (only `build` was gated before).
- CI gates engine + server with `gofmt -l` (blocking).
- All three workflows opt into Node 24 for JS actions
  (`FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true`) ahead of the Sep 2026
  forced cutover.

### Docs
- v0.1.0, v0.1.1, v0.1.2 GitHub Releases annotated as superseded —
  they were part of the bootstrap sequence; the signed-manifest +
  binary loop only closed cleanly at v0.1.3.

## [0.1.3] — 2026-06-07

### Fixed
- Pass `--skip=validate` to goreleaser inside `sign-manifest.yml`.
  `git checkout -- .` (tried in 0.1.2) did not undo `stark build`'s
  remove-then-write effects on Linux, so the clean-tree check kept
  firing. The binary build itself reads `engine/cmd/stark` from the
  tagged ref — not `dist/claude` — so skipping the validate step is safe
  and the release artifact is unaffected.

## [0.1.2] — 2026-06-07

### Fixed
- Restore working tree (`git checkout -- .`) before invoking goreleaser
  inside `sign-manifest.yml`. The previous run's `stark build` re-rendered
  `dist/claude` in place which tripped goreleaser's clean-tree check even
  when the rebuild was byte-identical. Binaries build from the tagged
  source, so the checkout is safe and unblocks the goreleaser stage.
  (Insufficient on Linux runners — superseded by 0.1.3.)

## [0.1.1] — 2026-06-07

### Fixed
- `stark` CLI binaries are now attached to signed releases. v0.1.0 shipped
  with the signed manifest but no binaries because the tag was pushed by
  `GITHUB_TOKEN`, which doesn't trigger downstream `on: push: tags`
  workflows. Folded goreleaser into `sign-manifest.yml` so every signed
  release atomically ships manifest + binaries. (v0.1.1 still missed the
  binaries due to a separate clean-tree bug fixed in v0.1.2.)

## [0.1.0] — 2026-06-07

First tagged release. Spec slices 1–8 complete (catalog → engine → web → security → web-deploy → governance).

### Added
- Canonical `catalog/` source-of-truth with 6 spec-kit-aligned bundles (`stark-constitution`, `stark-plan`, `stark-analyze`, `stark-implement`, `stark-gh`, `stark-ops`).
- Go engine (`engine/cmd/stark`) with `validate`, `build`, `check-bumps`, `lint`, `install`, `import`, `verify-manifest`, `doctor`, `info`, `search`, `version`.
- Per-runtime adapters for Claude Code, Codex, Gemini under `engine/internal/adapter/`.
- Web registry (`web/`) — strict-TS Vite SPA over signed `index.json` + `bundles/*.json`.
- IAP-gated Cloud Run static origin at `marketplace.evinced.rocks` (`server/`, `web-deploy.yml`).
- Native Claude Code marketplace via repo-root `.claude-plugin/marketplace.json`.
- CI gates: schema validate, drift `build --check`, version-bump immutability, gitleaks (fail-closed); body lint (advisory).
- Cosign-keyless signed build manifest via GitHub OIDC → Fulcio + Rekor.
- Top-level docs: `CLAUDE.md`, `AGENTS.md`, `README.md`, `CONTRIBUTING.md`, `docs/SECURITY.md`, `docs/native-install-loop.md`, `docs/web-hosting.md`.

[Unreleased]: https://github.com/GetEvinced/stark-marketplace/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/GetEvinced/stark-marketplace/releases/tag/v0.1.4
[0.1.3]: https://github.com/GetEvinced/stark-marketplace/releases/tag/v0.1.3
[0.1.2]: https://github.com/GetEvinced/stark-marketplace/releases/tag/v0.1.2
[0.1.1]: https://github.com/GetEvinced/stark-marketplace/releases/tag/v0.1.1
[0.1.0]: https://github.com/GetEvinced/stark-marketplace/releases/tag/v0.1.0
