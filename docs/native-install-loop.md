# Native Claude Code install loop

`stark-marketplace` installs into Claude Code with **no custom client**. The
committed repo-root `.claude-plugin/marketplace.json` IS the marketplace manifest;
CC reads it directly and resolves each plugin's `source` (e.g.
`./dist/claude/stark-gh`) relative to the marketplace root (the repo root).

> **Why the manifest is at the repo root, not under `dist/claude/`:** CC's
> `/plugin marketplace add owner/repo` shorthand looks for
> `.claude-plugin/marketplace.json` at the repository root, and relative plugin
> `source` paths resolve against the directory that contains `.claude-plugin/`.
> Committing the manifest at the repo root makes both the documented add command
> and the `./dist/claude/<bundle>` sources resolve. The bundle trees themselves
> stay under the committed `dist/claude/` tree.

## What is committed (spec §5.1)

- **Committed:** repo-root `.claude-plugin/marketplace.json`, the `dist/claude/`
  bundle trees, `index.json`, `bundles/*.json` — marked `linguist-generated`.
- **NOT committed:** `dist/codex/`, `dist/gemini/` — built on `stark install`
  (no in-repo consumer).

## End-to-end loop

1. Add the marketplace (private repo; you must have Evinced repo access):
   ```
   /plugin marketplace add GetEvinced/stark-marketplace
   ```
   CC resolves `dist/claude/.claude-plugin/marketplace.json` and lists every
   bundle as an installable plugin.

2. Install a bundle (one `plugins[]` entry == one bundle):
   ```
   /plugin install stark-gh
   ```
   CC fetches the plugin from the entry's `source` (`./dist/claude/stark-gh`)
   and installs its skills/commands/agents/mcp.

3. Update after a marketplace change:
   ```
   /plugin marketplace update GetEvinced/stark-marketplace
   /plugin install stark-gh
   ```

## Manifest contract (why installs resolve)

- Root carries `owner` (name/email).
- Each `plugins[]` entry carries `author` (NOT owner), `source`, `version`,
  `description`, `category`, `tags`, `strict`.
- `source` points at the bundle's committed `dist/claude/<bundle>/` tree (string
  form) — or an object form `{github|url|git-subdir}` when published from another
  repo.

The manifest is generated, never hand-edited: `stark build` regenerates it (it is
part of the generated `dist/claude` set); `stark build --check` fails CI on drift
(exit 2).

## Local verification

Run `docs/scripts/verify-native-install.sh` from the repo root. It rebuilds the
manifest, asserts the committed copy is drift-free, and structurally validates the
install contract (owner@root, author@entry, resolvable per-bundle source trees)
without needing a live CC session.
