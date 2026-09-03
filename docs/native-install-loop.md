# Native Claude Code install loop

`stark-marketplace` installs into Claude Code with **no custom client**. The
committed repo-root `.claude-plugin/marketplace.json` IS the marketplace manifest;
CC reads it directly and resolves each plugin's `source` (e.g.
`./dist/claude/stark-ops`) relative to the marketplace root (the repo root).

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
`standards/`, `scripts/`, `data/persona/`, `config.json`,
`forge_heuristics.json` — into every
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

### Codex installs vendor the same assets

`stark install --runtime codex` writes the vendored snapshot alongside the
skills, so a Codex install is self-contained the same way a Claude plugin is.
Layout under `--dest`:

| Path | Holds |
|---|---|
| `.agents/skills/<name>/SKILL.md` | the skill body (Codex-native discovery) |
| `.agents/skills/<name>/agents/openai.yaml` | UI metadata and explicit/implicit invocation policy |
| `.agents/skills/<name>/{references,scripts,assets}/**` | that skill's own supporting files |
| `.agents/stark/<bundle>/**` | the bundle's assets root — `tools/`, `standards/`, `prompts/`, `scripts/`, `data/persona/`, `config.json` |

The assets root is **per bundle, never shared**: `stark-ops` ships its own
`config.json`, which would clobber the shared snapshot's in a flat namespace.

Because the tree differs from a Claude plugin's, the codex target (`codex@3`)
retargets the reference shapes in each rendered body:

- `${CLAUDE_PLUGIN_ROOT}` / `${CLAUDE_PLUGIN_ROOT:-…}` →
  `${STARK_PLUGIN_ROOT:-$HOME/.agents/stark/<bundle>}`. The `$HOME` fallback
  assumes the recommended global install (`--dest ~`); for a project-local
  install, export `STARK_PLUGIN_ROOT` (see below).
- `${CLAUDE_PLUGIN_ROOT}/skills/<name>/…` addresses a **per-skill** asset, which
  lands at `.agents/skills/<name>/` (next to the skill), NOT under the bundle
  root — so it retargets via the bundle root's siblings
  (`${STARK_PLUGIN_ROOT:-…}/../../skills/…`), which resolves to `.agents/skills/`
  under both a global and a project-local `--dest`.
- `../{tools,standards,prompts,scripts}/` and `../../…` (any leading `../` run) →
  `../../stark/<bundle>/…`. On Claude a skill sits at
  `<plugin>/skills/<name>/SKILL.md` (`../../` **is** the plugin root) and a command
  at `<plugin>/commands/<name>.md` (`../`); on Codex both classes emit at
  `.agents/skills/<name>/SKILL.md`, so any `../` run is short of the bundle root.
- **Tool invocations** (`node --experimental-strip-types …/tools/x.ts`) are prefixed
  with an inline `STARK_ASSET_ROOT="${STARK_PLUGIN_ROOT:-…}"` export. A `${VAR:-…}`
  on the command line is shell *substitution*, not an export — so without this the
  tool's own `assetRoot()` (precedence `STARK_ASSET_ROOT` > `CLAUDE_PLUGIN_ROOT` >
  `~/.claude/code-review`) would resolve its sibling `config.json`/`prompts`/
  `standards`/tools to the Claude-only `~/.claude/code-review` a Codex install never
  creates. The inline assignment IS exported and inherited by every sibling tool the
  invocation spawns, so the whole process tree resolves to the vendored bundle root.

Vendored **prose** assets (references/standards/prompts markdown) get the
depth-independent half of this retarget too (var refs + tool invocations); their
relative `../` refs are left alone because a vendored file's on-disk depth differs
from a SKILL.md's.

**Project-local install (`--dest` other than `$HOME`).** The rendered bodies fall
back to `$HOME/.agents/stark/<bundle>`, but the assets are under `<dest>/.agents`.
`stark install --runtime codex` **warns** in this case and prints the exports to set
before invoking the skills, per bundle:

```
export STARK_PLUGIN_ROOT=<dest>/.agents/stark/<bundle>
export STARK_ASSET_ROOT=$STARK_PLUGIN_ROOT
```

The vendor roots default to `<repo>/vendor/stark-skills` and `<repo>/vendor/plugins`
(derived from `--catalog`'s parent). Override with `--assets-source` /
`--plugin-assets`; pass an **empty** value (`--assets-source ''`) to install
artifacts only. An explicit non-empty path that does not exist is a hard error, not
a silent asset-less install. Vendoring runs only for bundles that install an
asset-consuming artifact (skill/command/prompt/agent) — an mcp-only install writes
just its `.codex/config.toml` merge. MCP environment secrets use Codex's
`env_vars = ["NAME"]` forwarding contract; the adapter never writes an inert
literal `${NAME}` into `env`. The vendored asset step ships executable code
(`tools/*.ts`, `scripts/*.sh`) the skills invoke, so it participates in the §9.3
consent gate even when the bundle has no mcp/agent.
For HTTP MCP servers, canonical `env` is rejected rather than misrendered as
stdio `env_vars`; Codex HTTP authentication requires an explicit native
`bearer_token_env_var` or `env_http_headers` model.

**Still open:** `stark install --runtime claude` does not vendor assets — the
Claude distribution path is the committed `dist/claude/` plugin tree, which
already carries them. No bundle currently targets the `gemini` runtime — the
gemini adapter exists, but no bundle ships a gemini artifact today.

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
   /plugin marketplace add 21StarkCom/bifrost
   ```
   CC resolves `dist/claude/.claude-plugin/marketplace.json` and lists every
   bundle as an installable plugin.

2. Install a bundle (one `plugins[]` entry == one bundle):
   ```
   /plugin install stark-ops
   ```
   CC fetches the plugin from the entry's `source` (`./dist/claude/stark-ops`)
   and installs its skills/commands/agents/mcp.

3. Update after a marketplace change:
   ```
   /plugin marketplace update 21StarkCom/bifrost
   /plugin install stark-ops
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
