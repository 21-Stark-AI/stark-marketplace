# MCP command + agent.tools allowlists

Auto-generated from `engine/internal/validate/allowlist.go` and `engine/internal/validate/toolsallow.go`. **Do not hand-edit** — run `stark allowlist > docs/allowlist.md` after touching either source file (CI fails closed otherwise).

Governance: adding an entry requires a CODEOWNERS-gated PR with maintainer + `@aryeh-stark` approval. See [`SECURITY.md` §2](SECURITY.md).

## MCP `command` allowlist

MCP `command` values must be a basename present here. Every entry widens the set of binaries an MCP server may spawn on a developer's machine.

| Command |
| --- |
| `node` |
| `npx` |
| `stark-bq-mcp` |
| `stark-gh-mcp` |
| `uvx` |

## `agent.tools` allowlist

Tool grants on `agent` artifacts are surfaced for install-time consent. Unknown grants emit a `stark validate` warning (not a hard error) so reviewers see them in PR output.

| Tool |
| --- |
| `Bash` |
| `Edit` |
| `Glob` |
| `Grep` |
| `NotebookEdit` |
| `Read` |
| `Task` |
| `TodoWrite` |
| `WebFetch` |
| `WebSearch` |
| `Write` |
