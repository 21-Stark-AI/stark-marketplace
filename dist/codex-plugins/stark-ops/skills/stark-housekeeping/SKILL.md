---
name: stark-housekeeping
description: Audit and clean up stale issues, dead branches, and worktree remnants. Use for cleanup, housekeeping, close stale issues.
---
Usage: stark-housekeeping [--dry-run] [--repo ORG/REPO] [--aggressive]

## Codex plugin asset root

For every shell invocation that reads this skill's packaged files, first
resolve the absolute directory containing this loaded `SKILL.md` from the skill
path Codex supplied. In that same shell invocation set `SKILL_DIR` to that
directory, set `STARK_PLUGIN_ROOT` to the absolute `../..` directory, and
export it. Do not derive the plugin root from the current working directory and
do not reuse a value from an earlier shell invocation.

## Help

If the invocation arguments contain a standalone `--help`, `-h`, or `help` token,
follow [standard help](../../standards/help.md): print this skill's purpose,
usage, and arguments, then stop — do not run preflight or any phase.

# stark-housekeeping

Audits and cleans up project state: closes stale issues, deletes merged branches, removes worktree remnants, and reports remaining open work. Everything is presented before acting — no silent deletions.

## Arguments

| Arg | Default | Description |
|-----|---------|-------------|
| `--dry-run` | off | Show what would be cleaned, don't execute |
| `--repo ORG/REPO` | auto-detect | Override repo detection from git remote |
| `--aggressive` | off | Also close issues with no activity in 30+ days |

Parse options directly from the user's current request after the explicitly
invoked skill name.

## Constants

Detect the repo (or use `--repo` override) by parsing `org/repo` from
`git remote get-url origin`. Resolve bundled tools locally in each shell call;
do not rely on variables from an earlier call.
