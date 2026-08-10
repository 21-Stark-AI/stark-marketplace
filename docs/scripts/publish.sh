#!/usr/bin/env bash
set -euo pipefail

# publish.sh — regenerate the marketplace from stark-skills, the RIGHT way.
#
# Since bifrost's GitHub Actions are disabled for $0 spend, the old auto-publish
# `marketplace-sync` PR no longer fires — regeneration is manual. This chains the
# error-prone two-step regen (sync -> golden -> build --fix -> build --check) that
# the CLAUDE.md gotcha warns about, so you can't commit a `sync` without a `build`
# and trip the drift gate.
#
# SEMVER POLICY (applied automatically): every change ships a version bump.
#   - Adding OR removing a skill (bundle membership change) -> MINOR bump of that
#     bundle. The root VERSION also takes a MINOR.
#   - Any other content change (a stark-skills edit that re-renders artifacts)
#     -> PATCH bump of each affected bundle (default). Root VERSION takes PATCH.
# So a deploy never ships un-bumped content, and add/remove is signalled as minor.
#
# Usage:
#   docs/scripts/publish.sh                                     # sync + patch-bump changed
#   docs/scripts/publish.sh --add-skill stark-gha-cost --bundle stark-ops
#   docs/scripts/publish.sh --remove-skill stark-foo --bundle stark-ops
#   docs/scripts/publish.sh --add-skill X --bundle Y --ci --deploy
#
# Env:
#   STARK_SKILLS   path to the stark-skills checkout (default: ../../stark-skills
#                  relative to this repo, i.e. a sibling clone)
#
# It does NOT commit, push, or open a PR — that stays a deliberate human step
# (the script prints the next commands). --deploy runs deploy-web.sh at the end.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

STARK_SKILLS="${STARK_SKILLS:-$REPO_ROOT/../stark-skills}"
ADD_SKILL="" REMOVE_SKILL="" BUNDLE="" RUN_CI=0 RUN_DEPLOY=0
MEMBERSHIP_CHANGED=0

while [ $# -gt 0 ]; do
  case "$1" in
    --add-skill)    ADD_SKILL="${2:?}"; shift 2 ;;
    --remove-skill) REMOVE_SKILL="${2:?}"; shift 2 ;;
    --bundle)       BUNDLE="${2:?}"; shift 2 ;;
    --ci)           RUN_CI=1; shift ;;
    --deploy)       RUN_DEPLOY=1; shift ;;
    -h|--help)      sed -n '3,33p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

[ -d "$STARK_SKILLS/skill" ] || { echo "stark-skills not found at $STARK_SKILLS (set STARK_SKILLS)" >&2; exit 1; }
echo "→ stark-skills: $STARK_SKILLS"

# Skills deliberately NOT published to the marketplace (personal/experimental).
# A skill must be either a member of some bundle.yaml OR listed here — otherwise
# the coverage gate below fails. This is what stops a NEW stark-skills skill from
# being silently dropped by `stark sync` (which only pulls declared members).
EXCLUDED_SKILLS=(stark-voice)

# bump <file> <minor|patch> — handles both a bundle.yaml `version: X.Y.Z` line and
# the bare-`X.Y.Z` root VERSION file. minor: X.(Y+1).0 · patch: X.Y.(Z+1).
bump() {
  KIND="$2" perl -i -pe '
    my $k=$ENV{KIND};
    if (/^(version:\s*)(\d+)\.(\d+)\.(\d+)\s*$/) {
      $_ = $k eq "minor" ? "$1$2.".($3+1).".0\n" : "$1$2.$3.".($4+1)."\n";
    } elsif (/^(\d+)\.(\d+)\.(\d+)\s*$/) {
      $_ = $k eq "minor" ? "$1.".($2+1).".0\n" : "$1.$2.".($3+1)."\n";
    }
  ' "$1"
}

# --- optional: add / remove a skill in a bundle's membership (a MINOR change) ---
if [ -n "$ADD_SKILL$REMOVE_SKILL" ]; then
  [ -n "$BUNDLE" ] || { echo "--add-skill/--remove-skill require --bundle" >&2; exit 2; }
  BFILE="catalog/$BUNDLE/bundle.yaml"
  [ -f "$BFILE" ] || { echo "no such bundle: $BFILE" >&2; exit 1; }
fi

if [ -n "$ADD_SKILL" ]; then
  [ -d "$STARK_SKILLS/skill/$ADD_SKILL" ] || { echo "skill '$ADD_SKILL' not in stark-skills" >&2; exit 1; }
  if grep -qE "^[[:space:]]*-[[:space:]]+$ADD_SKILL[[:space:]]*$" "$BFILE"; then
    echo "→ $ADD_SKILL already in $BUNDLE — no membership change"
  else
    ADD_SKILL="$ADD_SKILL" perl -0pi -e '
      my $s=$ENV{ADD_SKILL};
      s/(^skills:\n(?:[ \t]*-[ \t]*\S+\n)+)/"$1  - $s\n"/me;
    ' "$BFILE"
    grep -qE "^[[:space:]]*-[[:space:]]+$ADD_SKILL[[:space:]]*$" "$BFILE" \
      || { echo "ERROR: failed to insert $ADD_SKILL into $BFILE (no skills: block?)" >&2; exit 1; }
    bump "$BFILE" minor; MEMBERSHIP_CHANGED=1
    echo "→ added $ADD_SKILL to $BUNDLE (minor) → $(grep -m1 '^version:' "$BFILE")"
  fi
fi

if [ -n "$REMOVE_SKILL" ]; then
  grep -qE "^[[:space:]]*-[[:space:]]+$REMOVE_SKILL[[:space:]]*$" "$BFILE" \
    || { echo "ERROR: $REMOVE_SKILL is not a member of $BUNDLE" >&2; exit 1; }
  REMOVE_SKILL="$REMOVE_SKILL" perl -ni -e 'print unless /^\s*-\s+\Q$ENV{REMOVE_SKILL}\E\s*$/' "$BFILE"
  bump "$BFILE" minor; MEMBERSHIP_CHANGED=1
  echo "→ removed $REMOVE_SKILL from $BUNDLE (minor) → $(grep -m1 '^version:' "$BFILE")"
fi

# --- coverage gate: every stark-skills skill must be claimed or excluded ------
# Kills the "new skill silently dropped" papercut: `stark sync` only pulls skills
# already declared in a bundle.yaml, so a freshly-added stark-skills skill with no
# membership vanishes without a trace. Fail loudly here instead.
claimed="$(grep -rhE '^[[:space:]]*-[[:space:]]+stark-' "$REPO_ROOT"/catalog/*/bundle.yaml 2>/dev/null | sed -E 's/^[[:space:]]*-[[:space:]]+//' | sort -u)"
orphans=""
for d in "$STARK_SKILLS"/skill/stark-*/; do
  s="$(basename "$d")"
  printf '%s\n' "$claimed" | grep -qxF "$s" && continue
  [[ " ${EXCLUDED_SKILLS[*]} " == *" $s "* ]] && continue
  orphans="$orphans $s"
done
if [ -n "$orphans" ]; then
  echo "ERROR: stark-skills skill(s) not published and not excluded:$orphans" >&2
  echo "  → add each to a bundle's catalog/<bundle>/bundle.yaml 'skills:' list" >&2
  echo "    (or run with --add-skill NAME --bundle B), OR add to EXCLUDED_SKILLS" >&2
  echo "    in this script if it is intentionally unpublished." >&2
  exit 1
fi
echo "→ coverage gate clean: every stark-skills skill is claimed or excluded"

# Root VERSION: the single bifrost deploy-level semver — bumped on every
# publish, and bumped BEFORE the build because the codex-plugin manifests bake
# the root VERSION into dist/. Bumping after the build (the original order)
# left dist exactly one version stale — a guaranteed drift-gate failure on
# every publish (hit live twice, 2026-08-10).
# MINOR if this run changed any bundle membership, else PATCH.
if [ "$MEMBERSHIP_CHANGED" -eq 1 ]; then bump VERSION minor; else bump VERSION patch; fi
echo "→ bifrost VERSION → $(cat VERSION)"

cd engine

echo "→ stark sync (regenerate catalog + vendor from stark-skills)"
go run ./cmd/stark sync --from "$STARK_SKILLS" ../catalog

# Auto-bump (PATCH) every bundle whose generated content changed. A cross-cutting
# change in stark-skills touches several bundles; check-bumps content-locks each
# artifact to its bundle version, so each affected bundle must bump. `sync` never
# changes membership (that only happens above via the flags), so these are always
# content-only → PATCH. Loop: detect → bump → re-sync until the gate is clean.
for round in 1 2 3 4 5; do
  cb_out="$(go run ./cmd/stark check-bumps ../catalog 2>&1 || true)"
  changed=$(printf '%s\n' "$cb_out" \
    | sed -nE 's#^[[:space:]]*-[[:space:]]+([a-z0-9-]+)/.*#\1#p' | sort -u)
  [ -z "$changed" ] && { echo "→ version-bump gate clean"; break; }
  echo "→ content changed in: $(echo "$changed" | tr '\n' ' ') — patch-bumping + re-syncing (round $round)"
  for b in $changed; do bump "$REPO_ROOT/catalog/$b/bundle.yaml" patch; done
  go run ./cmd/stark sync --from "$STARK_SKILLS" ../catalog >/dev/null
  [ "$round" = 5 ] && { echo "ERROR: check-bumps still failing after 5 rounds" >&2; exit 1; }
done

echo "→ refresh adapter golden (harmless if unchanged)"
UPDATE_GOLDEN=1 go test ./internal/adapter/... >/dev/null 2>&1 || true

echo "→ stark build --fix (regenerate dist / bundles / index)"
go run ./cmd/stark build --fix ../catalog

echo "→ stark build --check (drift gate)"
go run ./cmd/stark build --check ../catalog

echo "→ stark check-bumps (final gate)"
go run ./cmd/stark check-bumps ../catalog

cd "$REPO_ROOT"

if [ "$RUN_CI" -eq 1 ]; then
  echo "→ ci-local.sh (full local gate)"
  docs/scripts/ci-local.sh
fi

echo
echo "✅ regeneration complete + drift-clean."
echo "   Review + commit EVERYTHING (catalog + vendor + dist + golden + VERSION):"
echo "     git add -A && git commit && git push && gh pr create ..."
if [ "$RUN_DEPLOY" -eq 1 ]; then
  echo "→ deploy-web.sh"
  docs/scripts/deploy-web.sh
else
  echo "   Then deploy the web origin when ready:  docs/scripts/deploy-web.sh"
fi
