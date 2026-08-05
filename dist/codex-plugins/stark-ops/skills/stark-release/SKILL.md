---
name: stark-release
description: 'Cut a release: changelog review (auto-generating from git log if [Unreleased] is empty), version bump, git tag, GitHub Release. Use for release, tag, bump version.'
---
Usage: stark-release [patch|minor|major] (optional — auto-detected if omitted)

## Codex plugin asset root

For every shell invocation that reads this skill's packaged files, first
resolve the absolute directory containing this loaded `SKILL.md` from the skill
path Codex supplied. In that same shell invocation set `SKILL_DIR` to that
directory, set `STARK_PLUGIN_ROOT` to the absolute `../..` directory, and
export it. Do not derive the plugin root from the current working directory and
do not reuse a value from an earlier shell invocation.

## Help

If the current request asks for help (a standalone `--help`, `-h`, or `help` token),
follow [standard help](../../standards/help.md): print this skill's purpose,
usage, and arguments, then stop — do not run preflight or any phase.

# Release Management

Reviews accumulated changes in CHANGELOG.md, bumps the version, creates a git tag,
and optionally publishes a GitHub Release. Version source of truth is git tags (semver).

## Prerequisites

Must be on a clean main branch:

```bash
# Use user's PAT for all release operations (PRs, tags, releases show as user)
unset GH_TOKEN
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)

git checkout main && git pull --rebase origin main
git status --porcelain  # must be empty
```

Abort if uncommitted changes or not on main. Stash or commit first.

## Versioning Rules

- **Source of truth:** Git tags. Format: `v{major}.{minor}.{patch}` (e.g., `v0.1.3`).
- **Version file(s):** Auto-detect and update every supported version file found in the repo (Python, Node, Rust). If no version file exists, the git tag alone carries the version.
- **Baseline:** If no tags exist, baseline is `0.1.0` from pyproject.toml.
- **Bump semantics:**
  - `patch` — bug fixes, small corrections (0.1.2 → 0.1.3)
  - `minor` — new features, session deliverables (0.1.3 → 0.2.0)
  - `major` — breaking changes, major milestones (0.2.0 → 1.0.0)
