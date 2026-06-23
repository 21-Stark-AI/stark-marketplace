---
name: demo-skill
type: skill
description: Demo skill exercised only by the claude adapter seed-bundle golden fixture.
version: 9.9.9
maturity: stable
runtimes:
  - claude
model: opus
disable-model-invocation: true
allowed-tools:
  - Bash
  - Read
---
Demo skill body. Frozen fixture content — edit only to deliberately update the
rendering golden (then bump bundle.yaml's version and run UPDATE_GOLDEN=1).
