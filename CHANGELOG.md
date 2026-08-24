# Changelog

All notable changes to `stark-marketplace`. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [SemVer](https://semver.org/spec/v2.0.0.html). Bumping `VERSION` on `main` triggers a tag + signed release.

## [Unreleased]

### Fixed
<!-- stark-gh:pr-merge pr=197 runId=c5d7811a-fcfa-4dfe-b61b-dd6ff2336977 -->
- Unblocked native marketplace publication and packaged `stark-brain` as an Atlas MCP integration without restoring the retired `remember` skill.

### Added
<!-- stark-gh:pr-merge pr=196 runId=c1422f87-9818-4484-94b7-36bd3219bffb -->
- Publish native Codex skills for `stark-bury`, `stark-handoff`, and `simple-gate`.

## [0.15.2] — 2026-08-05

### Fixed
- Isolated native Codex runtime assets and mutable state from Claude: packaged
  assets resolve through `STARK_ASSET_ROOT`, while sessions, reviews, healer
  state, alerts, and housekeeping use `STARK_STATE_ROOT` under `~/.stark`.
- Rewrote known native skill references to Codex `$skill-name` invocation syntax
  in generated skill and plugin metadata without changing canonical Claude
  descriptions or generated Claude artifacts.
- Made Codex housekeeping operate only on Codex-owned state, preserve existing
  monthly archives, and safely archive repository filenames that resemble
  command-line options.

## [0.15.1] — 2026-08-05

### Fixed
- Restored the canonical Claude skill and command sources after the 0.15.0
  cross-runtime rewrite. Codex compatibility now lives in source-owned runtime
  overlays, and regression coverage proves those overlays cannot change
  `dist/claude/`, the Claude marketplace, bundle digests, or `index.json`.
- Added a native Codex repository marketplace with all eight 21 Stark plugins
  and all 30 skills. The three `stark-gh` commands are native skills, so
  `$cleanup` and `$pr-merge` are no longer lost to the legacy 4 KB command
  migration limit.
- Native Codex skills now resolve packaged assets from their loaded plugin
  location and fail closed when that location is unavailable, avoiding stale
  standalone-install fallbacks.
- Codex `$cleanup --dry-run` no longer fetches or prunes refs, merged branches
  with unique local commits require explicit `--force`, and GitHub workflow
  state defaults to the runtime-neutral `~/.stark/code-review` tree.

## [0.15.0] — 2026-08-05

### Added
- **Native Codex skill metadata.** `codex@3` emits
  `.agents/skills/<name>/agents/openai.yaml` with a normalized display name,
  25–64 character short description, and
  `policy.allow_implicit_invocation: false` for every explicit-only skill and
  command.
- **Install-level Codex contract coverage.** A dynamic test derives every
  committed Codex artifact from the index, installs every production bundle,
  and verifies exact inventory, frontmatter, metadata policy, support files,
  placeholder removal, and local-reference closure.
- `stark-voice` joins `stark-write`; all 27 canonical skills and the three
  `stark-gh` commands are now covered by the native Codex audit.

### Changed
- Codex no longer fabricates a model mapping for Claude model hints. Unsupported
  per-skill `model` metadata is dropped, while `disable-model-invocation` is
  derived into native Codex policy. Canonical skill `argument-hint` values are
  preserved as body-level usage prose.
- Per-skill support content now uses the standard `references/`, `scripts/`, and
  `assets/` directories. Shared persona data is vendored for installed persona
  and session flows; private `.remember` state is excluded from snapshots.
- Codex MCP configuration now installs to `.codex/config.toml`; stdio secret
  names render as sorted `env_vars` forwarding rather than literal
  `${ENV_KEY}` values. HTTP MCP entries reject ambiguous canonical `env`
  credentials instead of emitting stdio-only fields; native bearer/header
  credential shapes must be modeled explicitly before they can be rendered.
- The stark-skills source was audited for cross-runtime invocation, asset
  resolution, shell-state isolation, read-only defaults, explicit external
  side effects, runner sandboxing, and secret-safe GitHub identity switching.
  Terraform/Terragrunt provider dispatch, scanner execution, `.tfvars`
  inclusion, and posting now have separate consent gates.

### Removed
- The curated `stark-gh` MCP fragment that referenced the nonexistent
  `gh-mcp-server.js`. The bundle intentionally ships its three native commands;
  no broken MCP server is advertised.

### Fixed
- GitHub Actions cost probes no longer require Python, round billing per job,
  and account for Linux, Windows, and macOS multipliers.
- Jury, persona, session, handover, release, review, build, documentation, and
  GitHub command skills now resolve installed assets and support files in both
  source and native Codex layouts without `$ARGUMENTS` or persistent-shell
  assumptions.

## [0.14.0] — 2026-08-05

### Fixed — Codex installs were shipping dangling asset references
- **All 29 artifacts were broken on Codex.** `stark build` vendors the stark-skills snapshot (`tools/`, `standards/`, `prompts/`, `scripts/`, `config.json`) into every `dist/claude/<bundle>/`, but `stark install --runtime codex` wrote `SKILL.md` files and nothing else. Every one of them referenced assets that were never installed — 19 via `${CLAUDE_PLUGIN_ROOT}` (unset on Codex) and 26 via `../../standards/help.md`-style relative paths. Skills installed and were discoverable; anything that shelled a tool or followed a standard died.
- **`stark install --runtime codex` now vendors the same assets**, per bundle: `.agents/stark/<bundle>/{tools,standards,prompts,scripts,config.json}`, with each skill's own `references/` kept next to it at `.agents/skills/<name>/references/`. The assets root is per bundle, not shared — `stark-gh`'s `config.json` would otherwise clobber the shared snapshot's.
- **Codex target `codex@1` → `codex@2`**: rendered bodies are retargeted onto that tree — `${CLAUDE_PLUGIN_ROOT…}` → `${STARK_PLUGIN_ROOT:-$HOME/.agents/stark/<bundle>}`, `${CLAUDE_PLUGIN_ROOT}/skills/<name>/…` → the `.agents/skills/` root, and any leading `../` run before `{tools,standards,prompts,scripts}/` → `../../stark/<bundle>/…`.

### Fixed — review findings on the codex@2 vendoring (same PR)
- **Vendored tools resolved their assets to the wrong root.** A retargeted body ran `node …/tools/x.ts`, but the tool's own `assetRoot()` (`STARK_ASSET_ROOT` > `CLAUDE_PLUGIN_ROOT` > `~/.claude/code-review`) read `config.json`/`prompts`/`standards`/sibling-tools from the Claude-only `~/.claude/code-review` — a path a Codex install never creates. Tool invocations now carry an inline `STARK_ASSET_ROOT="${STARK_PLUGIN_ROOT:-…}"` export (inherited by the whole process tree), so the tools are actually self-contained.
- **The live gate now runs a tool** (`asset_root_lib` resolution under the installed tree), not just `os.Stat` of paths named in `SKILL.md` — the previous check was false-green against the resolution bug above.
- **Project-local install** (`--dest` ≠ `$HOME`) now **warns** and prints the `STARK_PLUGIN_ROOT`/`STARK_ASSET_ROOT` exports to set, instead of silently dangling on the baked `$HOME` fallback.
- **A stale unmanaged file under `.agents/stark/<bundle>/` no longer aborts the whole install** — the installer-owned asset step is exempt from the unmanaged-collision preflight and overwrites (still journaled + removable).
- **`--assets-source`/`--plugin-assets` validate:** an explicit non-existent path is a hard error (was a silent asset-less install), and an explicit empty value disables vendoring (was overridden back to the default).
- **An mcp-only install no longer over-vendors** the bundle's ~150-file asset tree — vendoring runs only for asset-consuming artifacts (skill/command/prompt/agent).
- **Executable vendored assets are consented:** `tools/*.ts` + `scripts/*.sh` set `Consent.Required` and list under `Consent.AssetExec` (§9.3) even when the bundle ships no mcp/agent.
- **Retarget coverage:** command/prompt single-`../` refs, per-skill `${CLAUDE_PLUGIN_ROOT}/skills/` refs, and `${CLAUDE_PLUGIN_ROOT}`/tool refs in vendored prose markdown are now all handled.

### Added
- **`installplan.AssetProvider`** — optional adapter interface for bundle-level files that belong to no artifact. Its step is prepended (artifacts win on collision) and kept out of the artifact `ClosureRefs`; only bundles that install an asset-consuming artifact get one. It participates in the §9.3 consent gate when it carries executable code.
- `stark install --assets-source` / `--plugin-assets`, defaulting to `<repo>/vendor/stark-skills` and `<repo>/vendor/plugins` off `--catalog`'s parent.

### Not covered
- `stark install --runtime claude` still vendors nothing — its distribution path is the committed `dist/claude/` plugin tree, which already carries the assets.
- Gemini remains `stark-gh`-only (3 commands). The other 26 skills declare `[claude, codex]`; rendering them as `GEMINI.md` sentinel blocks would put ~307 KB permanently in context, so that is deliberately deferred.

## [0.8.0] — 2026-07-26

### Removed — the demolition release
- **11 skills deleted** with the stark-skills loop-machinery demolition (autopsy 2026-07-25; stark-skills #802/#803/#804, ~35k LOC): **stark-analyze** drops `stark-write-spec`, `stark-review-spec`, `stark-review-plan`, `stark-review-spec-improvement`, `stark-red-team-spec`, `stark-red-team-plan`, `stark-red-team-fold` (`0.4.2 → 0.5.0`); **stark-plan** drops `stark-spec-to-plan`, `stark-plan-to-tasks` (`0.2.2 → 0.3.0`, now carries `stark-author` alone); **stark-implement** drops `stark-phase-execute`, `stark-forge` (`0.3.2 → 0.4.0`, carries `stark-build` + `stark-copilot`).
- The two-stage pipeline is the replacement: `/stark-author` (human-gated spec+plan) → `/stark-build` (check-gated implementation). No LLM-reviews-LLM loops anywhere.

### Changed
- **stark-ops** `0.2.8 → 0.2.9` (PATCH — `stark-gh-user` prose repointed at `stark_review.ts`).
- Root `VERSION` `0.7.0 → 0.8.0` (MINOR — bundle membership changed).

## [0.7.0] — 2026-07-26

### Added
- **`stark-fresh-eyes`** skill in the **stark-analyze** bundle — one-shot zero-context review of a prompt/brief/spec/doc: a single read-only subagent re-verifies every checkable claim by a DIFFERENT method (recursive recounts, recompute from raw sources, path resolution, `--help` on cited commands) and reports defects only; the author dispositions findings once — never a round 2. Pure protocol skill, zero TS.

### Changed
- **stark-analyze** MINOR bump (membership changed); root `VERSION` `0.6.0 → 0.7.0`.

## [0.6.0] — 2026-07-25

### Added
- **`stark-author`** skill in the **stark-plan** bundle (`0.1.11 → 0.2.0`) — **Stage 1 of the two-stage rebuild**: human-gated spec+plan authoring in ONE session (tier check → time-boxed recon → structured interview → one self-contained spec+plan doc + plain-English `.human.md` sidecar → one zero-context advisory pass → 8-item human gate → `accepted-base` pin → draft PR). No LLM-reviews-LLM loop anywhere. For new work it replaces the write-spec → review-spec → red-team-spec → spec-to-plan → review-plan chain.
- **`stark-build`** skill in the **stark-implement** bundle (`0.2.0 → 0.3.0`) — **Stage 2**: autonomous implementation from an accepted stark-author spec. One fresh headless session per task, gated by checks the agent cannot edit (PreToolUse path-deny + Stop-hook gate with 7-block abort-with-deviation), commit per green task, held-out e2e gate, ONE cross-vendor advisory review whose findings die at the human. Zero new TS — protocol skill + two POSIX hook scripts.

### Changed
- **stark-plan** `0.1.11 → 0.2.0`, **stark-implement** `0.2.0 → 0.3.0` (MINOR — bundle membership changed).
- Root `VERSION` `0.5.0 → 0.6.0` (MINOR — bundle membership changed).

## [0.5.0] — 2026-07-25

### Added
- **`stark-forge`** skill in the **stark-implement** bundle (`0.1.12 → 0.2.0`) — the pipeline **conductor**. Chains the six pipeline stages (8 with `--red-team`) in-session over a crash-resumable, merge-at-artifact-boundaries state machine (`tools/forge_state{,_lib}.ts`); resolves chain/merge-points/commands and base-sync routing itself, so the skill is glue. Bundle now carries the full autonomous-execution trio: `stark-copilot` + `stark-phase-execute` + `stark-forge`.

### Changed
- **stark-implement** `0.1.12 → 0.2.0` (MINOR — bundle membership changed) — also re-renders `stark-copilot` (now lands an impl PR via `tools/copilot_land.ts` — create-or-adopt branch, push never `--force`, draft-by-default) and picks up the repo-wide `claude-opus-4-8 → claude-opus-5[1m]` default.
- Root `VERSION` `0.4.0 → 0.5.0` (MINOR — bundle membership changed).

## [0.4.0] — 2026-07-18

### Added
- **`stark-write-spec`** skill in the **stark-analyze** bundle (`0.2.0 → 0.3.0`) — contract-bounded spec authoring, pipeline stage 0 (before `/stark-review-spec`). A bounded lead/wing loop drafts and verifies a nine-section spec against a host-owned closed-enum contract; `done` is recomputed host-side. Bundle now spans spec-kit's specify + analyze phases.

### Changed
- **stark-ops** `0.2.1 → 0.2.2` — absorbs pre-existing `stark-housekeeping` source drift surfaced by a full re-sync.
- Root `VERSION` `0.3.0 → 0.4.0` (MINOR — bundle membership changed).

_Note: `0.2.x`–`0.3.0` shipped as script-based publishes after GitHub Actions were disabled for $0 spend; git tags + signed releases remain paused at v0.1.6. `/plugin` consumers read `main` directly._

## [0.1.6] — 2026-06-07

### Fixed
- Gitignore the `sign-manifest.yml` scratch files
  (`build-manifest.json{,.sig,.pem}`, `build-manifest.sha256`,
  `release-notes.md`). With the `dist/claude` collision fixed in
  v0.1.5, these untracked artifacts were the last thing keeping
  goreleaser's clean-tree check unhappy. v0.1.5 itself shipped a
  signed manifest but no binaries; v0.1.6 is the first release where
  every assertion in the plan ships without workarounds.

## [0.1.5] — 2026-06-07

### Fixed
- Root cause for the v0.1.1–v0.1.3 "git dirty state" identified and
  fixed: `goreleaser release --clean` was wiping its default `./dist`
  directory, which collides with the repo's committed `dist/claude`
  tree. `.goreleaser.yaml` now sets `dist: .goreleaser-dist`, so the
  two never touch. `--skip=validate` removed from `sign-manifest.yml`;
  the validate gate now passes legitimately.
- Diagnostic step in `sign-manifest.yml` removed (served its purpose —
  see v0.1.4 run 27098850309 in workflow history for the captured
  state that exposed the cause).

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
- IAP-gated Cloud Run static origin at `marketplace.21stark.com` (`server/`, `web-deploy.yml`).
- Native Claude Code marketplace via repo-root `.claude-plugin/marketplace.json`.
- CI gates: schema validate, drift `build --check`, version-bump immutability, gitleaks (fail-closed); body lint (advisory).
- Cosign-keyless signed build manifest via GitHub OIDC → Fulcio + Rekor.
- Top-level docs: `CLAUDE.md`, `AGENTS.md`, `README.md`, `CONTRIBUTING.md`, `docs/SECURITY.md`, `docs/native-install-loop.md`, `docs/web-hosting.md`.

[Unreleased]: https://github.com/21-Stark-AI/stark-marketplace/compare/v0.1.6...HEAD
[0.1.6]: https://github.com/21-Stark-AI/stark-marketplace/releases/tag/v0.1.6
[0.1.5]: https://github.com/21-Stark-AI/stark-marketplace/releases/tag/v0.1.5
[0.1.4]: https://github.com/21-Stark-AI/stark-marketplace/releases/tag/v0.1.4
[0.1.3]: https://github.com/21-Stark-AI/stark-marketplace/releases/tag/v0.1.3
[0.1.2]: https://github.com/21-Stark-AI/stark-marketplace/releases/tag/v0.1.2
[0.1.1]: https://github.com/21-Stark-AI/stark-marketplace/releases/tag/v0.1.1
[0.1.0]: https://github.com/21-Stark-AI/stark-marketplace/releases/tag/v0.1.0
