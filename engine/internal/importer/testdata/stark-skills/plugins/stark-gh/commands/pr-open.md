---
name: pr-open
description: >-
  Open or update a PR with Codex-drafted prose, stage-all commit, push, and CI watcher.
argument-hint: "[--draft] [--base BRANCH] [--reviewer LIST]"
allowed-tools: Bash, Read
model: sonnet
---

# /stark-gh:pr-open

Open or update a GitHub pull request through a fixed three-stage pipeline:
preflight, draft, execute.
