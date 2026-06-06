---
name: demo-review
description: >-
  Single-agent PR review. Uses triage-selected PR review domains by default,
  or one forced agent via `--agent`.
argument-hint: "[PR_NUMBER] [--agent claude|codex|gemini] [--quick]"
disable-model-invocation: false
model: opus[1m]
revision: 7d4eb375d131624ff59927945d448856858d621c
revision_date: 2026-05-18T16:33:25Z
---

Single-agent PR review path. Keep this skill thin: do preflight, capture the
trusted config root, set up the worktree, then hand off to the dispatcher.

## Arguments

Raw input: `$ARGUMENTS`
