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

## Self-contained plugins (no install.sh on the target)

Each `dist/claude/<bundle>/` is a **self-contained** Claude Code plugin. `stark
build` vendors the immutable stark-skills assets — `tools/`, `prompts/`,
`standards/`, `scripts/`, `config.json`, `forge_heuristics.json` — into every
bundle and emits a per-bundle `.claude-plugin/plugin.json`. Skills resolve their
tool/config/prompt paths through `${CLAUDE_PLUGIN_ROOT}` (set by CC to the
installed plugin dir), falling back to `~/.claude/code-review` only in local
stark-skills dev (install.sh symlinks). Mutable state (`history/`, `sessions/`,
…) always stays under `$HOME`, never inside the plugin cache.

Net effect: a teammate runs `/plugin install <bundle>` and everything works —
**no stark-skills checkout, no install.sh** required on their machine.

### Runtime prerequisite

Skills shell out to the vendored tools via `node --experimental-strip-types`,
which needs **Node ≥ 22.6** (24+ recommended). `stark doctor` verifies this.

## What is committed (spec §5.1)

- **Committed:** repo-root `.claude-plugin/marketplace.json`, the `dist/claude/`
  bundle trees (incl. vendored `tools/`/`prompts/`/`config.json` + per-bundle
  `.claude-plugin/plugin.json`), `index.json`, `bundles/*.json`, and the
  `vendor/stark-skills/` asset snapshot — all marked `linguist-generated`.
- **NOT committed:** `dist/codex/`, `dist/gemini/` — built on `stark install`
  (no in-repo consumer).

## Generation pipeline (catalog is generated from stark-skills)

stark-skills is the single source of truth; the catalog is generated, so the two
repos cannot drift:

```
stark sync --from <stark-skills checkout>   # regenerate catalog/<bundle>/{skills,commands}
                                            # + refresh vendor/stark-skills snapshot
stark build                                 # render dist/ (vendors the snapshot) + manifests
```

- Membership is declared per bundle in `catalog/<bundle>/bundle.yaml`
  (`skills:` / `commands:`); `stark sync` pulls exactly those from the checkout.
  `mcp/` artifacts are **curated** in the catalog (stark-skills defines none).
- Artifacts inherit their **bundle's** `version` + `runtimes`. To publish a
  content change, bump the bundle `version` in `bundle.yaml` (one place) — this
  satisfies the per-artifact version-bump gate (`stark check-bumps`).
- `stark sync --check` is the cross-repo drift gate (committed catalog/vendor vs
  a fresh generation); `stark build --check` is the catalog→dist drift gate.

These two repos are wired together by CI (`.github/workflows/marketplace-sync.yml`
in **stark-skills**): a push to stark-skills `main` regenerates and opens a PR
here automatically.

## End-to-end loop

1. Add the marketplace (private repo; you must have 21 Stark AI repo access):
   ```
   /plugin marketplace add 21-Stark-AI/stark-marketplace
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
   /plugin marketplace update 21-Stark-AI/stark-marketplace
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
