---
name: cleanup
description: >-
  Sweep the repo for merged-PR branches, stale refs, worktree leftovers, and
  loose objects; rebase onto upstream.
argument-hint: "[--dry-run] [--force] [--json]"
allowed-tools: Bash
model: sonnet
---

# /stark-gh:cleanup

Sweep local and remote git state and rebase the current branch.
