---
name: stark-session
description: Session start (context, git state, briefing) and end (tests, merge, push). Use for session start/end, catch me up.
---
Usage: stark-session [start|end]

## Codex plugin asset root

For every shell invocation that reads this skill's packaged files, first
resolve the absolute directory containing this loaded `SKILL.md` from the skill
path Codex supplied. In that same shell invocation set `SKILL_DIR` to that
directory, set `STARK_PLUGIN_ROOT` to the absolute `../..` directory, and
export it. Do not derive the plugin root from the current working directory and
do not reuse a value from an earlier shell invocation.

## Help

If the current user request includes a standalone `--help`, `-h`, or `help` token,
follow [standard help](../../standards/help.md): print this skill's purpose,
usage, and arguments, then stop — do not run preflight or any phase.

## Preflight

Run [standard preflight](../../standards/preflight.md) with `--workflow stark-session`.

# stark-session

Session lifecycle management: **start** (context load + briefing) and **end** (test + merge + commit + push).

A single TS CLI gathers every fact you need into one JSON blob; you render the briefing/summary directly. No ANSI, no box-drawing, no fallback path — when a collector fails, its slot is `null` and the failure is logged into `errors[]`.

## Arguments

- no input or `start` — starts a session (default)
- `end` — ends a session

Read the mode from the current user's explicit invocation. Do not depend on a
host-populated argument placeholder.

## Execution rule

Shell state does not persist between tool calls. Every command below resolves
`TOOLS` and consumes any variables in the same shell call. Resolve session
identity from `STARK_SESSION_ID`, the active host's thread/session variable, or
`session_id.ts`, in that order. Start time and start HEAD are persisted through
`session_state.ts`; end mode reads that state instead of expecting variables
from start mode to survive.

## Config

Path: `.code-review/config.json` (hierarchical: global → org → repo). Reading is handled inside the TS CLI; you only need to know the keys that affect **end-mode dialogue**:

| Key | Default | Notes |
|-----|---------|-------|
| `build_command` | `null` | Build command for end |
| `test_command` | `null` | Falls back to top-level |
| `doc_paths` | `["docs/", "AGENTS.md", "CLAUDE.md"]` | Paths to stage on end |
| `devlog_path` | `null` | Devlog directory |
| `pr_merge_strategy` | `"squash"` | squash/merge/rebase |

`session.health_checks` is consumed by the TS CLI directly — you don't run these yourself.
