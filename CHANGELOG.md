# Changelog

All notable changes to `stark-marketplace`. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [SemVer](https://semver.org/spec/v2.0.0.html). Bumping `VERSION` on `main` triggers a tag + signed release.

## [Unreleased]

## [0.1.1] — 2026-06-07

### Fixed
- `stark` CLI binaries are now attached to signed releases. v0.1.0 shipped
  with the signed manifest but no binaries because the tag was pushed by
  `GITHUB_TOKEN`, which doesn't trigger downstream `on: push: tags`
  workflows. Folded goreleaser into `sign-manifest.yml` so every signed
  release atomically ships manifest + binaries.

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

[Unreleased]: https://github.com/GetEvinced/stark-marketplace/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/GetEvinced/stark-marketplace/releases/tag/v0.1.1
[0.1.0]: https://github.com/GetEvinced/stark-marketplace/releases/tag/v0.1.0
