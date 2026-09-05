#!/usr/bin/env bash
set -euo pipefail

# ci-local.sh — run the full CI gate locally.
#
# A local mirror of `.github/workflows/ci.yml`. GitHub Actions ARE enabled and
# `ci` gates every PR — run this before pushing when you want the gate without
# waiting on CI (a fast local pre-flight), not because CI is off.
#
# Mirrors ci.yml exactly: engine (fmt/vet/test/validate/drift/bumps/lint/
# allowlist), web (typecheck/lint/test/build), server (fmt/vet/test),
# gitleaks secret scan, and actionlint. Any failure exits non-zero.
#
# Usage:  docs/scripts/ci-local.sh

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

fail=0
step() { echo; echo "==> $*"; }
softmiss() { echo "    (skipped: $* not installed)"; }

# ── engine ───────────────────────────────────────────────────────────
step "engine: gofmt"
out="$(cd engine && gofmt -l .)"; if [ -n "$out" ]; then echo "unformatted:"; echo "$out"; fail=1; fi
step "engine: go vet";                (cd engine && go vet ./...) || fail=1
step "engine: go test";               (cd engine && go test ./... -count=1) || fail=1
step "engine: stark validate";        (cd engine && go run ./cmd/stark validate ../catalog) || fail=1
step "engine: stark build --check";   (cd engine && go run ./cmd/stark build --check ../catalog) || fail=1
step "engine: stark check-bumps";     (cd engine && go run ./cmd/stark check-bumps ../catalog) || fail=1
step "engine: stark lint --strict";   (cd engine && go run ./cmd/stark lint --strict ../catalog) || fail=1
step "engine: stark allowlist --check"; (cd engine && go run ./cmd/stark allowlist --check ../docs/allowlist.md) || fail=1

# ── web ──────────────────────────────────────────────────────────────
step "web: npm ci";        (cd web && npm ci) || fail=1
step "web: typecheck";     (cd web && npm run typecheck) || fail=1
step "web: eslint";        (cd web && npm run lint) || fail=1
step "web: vitest";        (cd web && npm test) || fail=1
step "web: build";         (cd web && npm run build) || fail=1

# ── server ───────────────────────────────────────────────────────────
step "server: gofmt"
out="$(cd server && gofmt -l .)"; if [ -n "$out" ]; then echo "unformatted:"; echo "$out"; fail=1; fi
step "server: go vet";     (cd server && go vet ./...) || fail=1
step "server: go test";    (cd server && go test ./... -count=1) || fail=1

# ── secret scan ──────────────────────────────────────────────────────
step "gitleaks (working tree)"
if command -v gitleaks >/dev/null; then
  gitleaks dir . --config .gitleaks.toml --redact --exit-code 1 --verbose || fail=1
else softmiss gitleaks; fi

# ── actionlint ───────────────────────────────────────────────────────
step "actionlint"
if command -v actionlint >/dev/null; then
  actionlint || fail=1
else softmiss actionlint; fi

echo
if [ "$fail" -ne 0 ]; then echo "❌ ci-local: FAILURES above"; exit 1; fi
echo "✅ ci-local: all checks passed"
