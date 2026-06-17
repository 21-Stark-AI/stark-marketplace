# Repository Guidelines

## Project Structure & Module Organization

The **stark-skills** repo is the source of truth. Here, `catalog/<bundle>/bundle.yaml` (metadata + `skills:`/`commands:` membership) and `catalog/<bundle>/mcp/` are **curated**; `catalog/<bundle>/{skills,commands}` and `vendor/stark-skills/` are **generated** by `go run ./cmd/stark sync --from <stark-skills> ../catalog` (do not hand-edit). Generated artifacts also include `index.json`, `bundles/*.json`, `dist/claude/**`, and `.claude-plugin/**`; regenerate via `stark build` instead of hand-editing. `dist/codex/` and `dist/gemini/` are ignored install outputs. `schema/` holds public JSON schemas. `engine/` is the main Go module: CLI code is in `engine/cmd/stark`, packages in `engine/internal`. `server/` is the static-origin Go module. `web/` is the TypeScript Vite SPA; code is in `web/src`, fixtures in `web/src/__fixtures__`.

## Build, Test, and Development Commands

- `cd engine && go test ./... -count=1 && go vet ./...`: engine test/vet.
- `cd engine && go run ./cmd/stark sync --from ../../stark-skills ../catalog`: regenerate catalog skills/commands + vendor snapshot from stark-skills (`--check` = drift gate).
- `cd engine && go run ./cmd/stark validate ../catalog`: catalog validation.
- `cd engine && go run ./cmd/stark build --check ../catalog`: drift check.
- `cd engine && go run ./cmd/stark build --fix ../catalog`: regenerate artifacts.
- `cd engine && go run ./cmd/stark check-bumps ../catalog`: version immutability.
- `cd engine && go run ./cmd/stark install stark-gh/pr-open --runtime codex --dest /tmp/stark-codex --plan --index ../index.json --bundles ../bundles --catalog ../catalog`: preview Codex output.
- `cd server && go test ./... -count=1 && go vet ./...`: server test/vet.
- `cd web && npm ci && npm run dev`: install dependencies and start Vite.
- `cd web && npm test && npm run lint && npm run build`: Vitest, ESLint, build.

## Coding Style & Naming Conventions

Use `gofmt` and `go vet`; Go package names stay lowercase. TypeScript is strict and ESLint forbids `any`; prefer `web/src/types` models. React components use `PascalCase`, utilities use `camelCase`, and catalog IDs use kebab-case. Keep generated files deterministic and LF-only.

## Testing Guidelines

Go tests use `_test.go` files beside packages. Web tests use Vitest with `.test.ts` or `.test.tsx`. For catalog/schema edits, run validation, build drift, and bump checks. For Codex adapter changes, update `engine/internal/adapter/codex/testdata/*.golden` only with `go test ./internal/adapter/codex -update`, then rerun normal tests.

## Codex Agent Notes

Prefer editing `catalog/`, `engine/`, `server/`, or `web/src/` over generated outputs. Codex skills render to `.agents/skills/<name>/SKILL.md`; commands, prompts, and agents become skills, while MCP fragments merge into `config.toml`. Never commit local install outputs such as `.agents/`, `.codex/`, `.stark/`, or `config.toml`. MCP secrets stay `${ENV_KEY}` placeholders.

## Commit, PR, and Security Guidelines

Recent commits use scope or slice prefixes: `Slice 8: Security hardening...` and `web-deploy: ...`. PRs should describe changes, list validation commands, link issues, include web screenshots, and call out generated artifacts. Do not commit secrets; CI scans the tree and PR history with gitleaks.
