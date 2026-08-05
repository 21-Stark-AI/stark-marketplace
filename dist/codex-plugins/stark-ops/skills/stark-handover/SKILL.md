---
name: stark-handover
description: 'Use when pausing or splitting work across sessions — before clearing context, when context runs low, at end of day, when switching tasks — or when resuming. Triggers: "handover", "handoff", "save context", "save progress", "resume", "continue where we left off", "what was I doing". Persists a numbered handover chain + PROGRESS.md tracker per task; resume needs no recap.'
---
Usage: stark-handover [save|resume|status] [--task slug]

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

# stark-handover

Cross-session continuity. Every **save** appends `handover_{N}.md`
(numbered chain) and rewrites `PROGRESS.md` (done-vs-todo tracker) under
`{root}/{project}/{worktree}/{task}/`; **resume** loads both in one call so a
fresh session continues without a recap. Root default: `~/Code/Handovers`
(config `handover.root`, env `STARK_HANDOVER_ROOT`).

The CLI owns paths/numbering/writes; **you** author the content — the value
of a handover is what you mine from the conversation, which only you have.

## Execution rule

Shell state does not persist between tool calls. Every command below resolves
`TOOLS` and invokes `stark_handover.ts` in the same shell call; do not create a
function or command variable and expect a later call to inherit it.

## Arguments

- no input or `save` — save a handover (default)
- `resume` — resume the latest task; add `--task <slug>` to select one
- `status` — list this project/worktree's tasks

Read the mode and flags from the current user's explicit invocation. Do not
depend on a host-populated argument placeholder.

## Guards

- **Never write handover files freeform.** Ad-hoc summaries skip chain
  numbering, frontmatter, and the tracker — always go through the CLI.
- **Save only when asked** (explicit invocation or a clear "save context /
  wrap up" request). Don't burn tokens on speculative handovers.
- Not in plan mode — this skill writes files.
