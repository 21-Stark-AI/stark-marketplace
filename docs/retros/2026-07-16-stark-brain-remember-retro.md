# Retro: shipping `stark-brain` — first new-bundle registration (2026-07-16)

Registered the stark-skills `remember` skill as a new marketplace plugin,
`stark-brain` v0.1.0 — the first bundle created from scratch since the
publish tooling landed. This retro records the end-to-end path and the four
gotchas that cost time, so the next new-bundle registration is a 20-minute
job instead of an afternoon.

## What shipped

| Piece | Where |
|---|---|
| Skill reword (unblocked strict lint) | stark-skills [#680](https://github.com/21StarkCom/stark-skills/pull/680) → `e9b5db9` |
| New `catalog/stark-brain/bundle.yaml` (`skills: [remember]`) + full regen | bifrost [#90](https://github.com/21StarkCom/bifrost/pull/90) → `113d9c2` |
| Versions | bundle `0.1.0`; root `VERSION` `0.1.7 → 0.2.0` (membership = MINOR) |
| Local install | `claude plugin marketplace update bifrost` → `claude plugin install stark-brain@bifrost` |
| Web origin | `docs/scripts/deploy-web.sh` → Cloud Run `stark-marketplace-00029-mqh` (`ev-infra-group`); verified `bifrost.21stark.com/index.json` + `/bundles/stark-brain.json` serve 0.1.0 |
| GAR prune | repo policy (near-zero storage): deleted the 6 stale 07-11 digests, kept today's build |

The skill surfaces as `/stark-brain:remember`; it carries
`disable-model-invocation: true`, so it is user-invoked only. It is distinct
from Claude Code's *native* `/remember` file-memory (a harness built-in, not a
plugin) — the two coexist.

## Gotchas (the reusable part)

1. **`publish.sh --add-skill` cannot seed a bundle's FIRST skill.** Its perl
   insert anchors on an existing `skills:` block with at least one member —
   on a brand-new bundle there is nothing to anchor to. For a new bundle:
   hand-write `catalog/<bundle>/bundle.yaml` (schema:
   `schema/bundle.schema.json`; required `name`/`version`/`description`/`owner`)
   with the member skill already listed, then run the pipeline manually:
   `stark sync` → `UPDATE_GOLDEN=1 go test ./internal/adapter/...` →
   `stark build --fix` → `build --check` → `check-bumps` → `lint --strict` →
   bump root `VERSION` (minor) → `ci-local.sh`.

2. **`lint --strict` fail-closes on `[secret-file-read]` for any body that
   literally mentions `.env`, `.aws/credentials`, etc.** — even
   when the mention is *anti-secret guidance* (the `remember` skill's "record
   the pointer, never the value" section named the retired credential store as the pointer
   target). There is **no suppression/escape hatch**
   (`internal/validate/rules_lint.go`). Chosen fix: reword the skill upstream
   (portable phrasing beats an allowlist); considered and rejected an inline
   lint-allow marker and a narrower read-intent regex.

3. **A huge "drift" after `stark sync` is usually fake.** Two causes seen in
   one session: (a) the local stark-skills checkout was on a dirty feature
   branch, not `main`; (b) the bifrost working branch was based on a stale
   `main` (sync PRs #82→#89 had already landed the "drift"). Rule: **sync
   only from stark-skills `main`, and rebase onto bifrost `origin/main`
   first.** A correct new-bundle diff touches only `catalog/<bundle>/`,
   `dist/claude/<bundle>/`, `bundles/<bundle>.json`, `index.json`,
   `.claude-plugin/marketplace.json`, `VERSION` — anything else is a red flag.

4. **GAR prune is a two-pass delete.** buildx pushes a manifest *list*; GAR
   refuses to delete child manifests while a parent references them
   (`manifest is referenced by parent manifests`). Delete the tagged parents
   first, then the now-orphaned children. Also: a "current" deploy is a trio
   (tagged image + 2 provenance/manifest children sharing its timestamp) —
   keep all three.

## Sequence that worked (new-bundle recipe)

```text
stark-skills: fix/reword the skill if lint-dirty → PR → merge to main
bifrost:      git checkout main && pull → branch
              write catalog/<bundle>/bundle.yaml (members listed)
              engine: sync → golden → build --fix → build --check → check-bumps → lint --strict
              bump root VERSION (minor) → docs/scripts/ci-local.sh → PR → merge
local:        claude plugin marketplace update bifrost
              claude plugin install <bundle>@bifrost   (restart CC to load)
web (optional): docs/scripts/deploy-web.sh → verify index.json → prune GAR (two-pass)
```

Note: the stark-skills `marketplace-sync` GHA auto-opens a bifrost catch-up PR
on skill *content* changes to existing bundles — but bundle *membership* and
new bundles are curated here, so this flow stays manual by design.
