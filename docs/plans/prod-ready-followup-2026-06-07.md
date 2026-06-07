# /goal prompt — stark-marketplace prod-ready follow-up (2026-06-07)

Single self-contained prompt to feed into `/goal`. Each numbered item is independent — work them in any order, commit in slices, push to `main` (worktree → ff-only → push), and re-verify CI green before moving on.

---

You are continuing prod-readiness work on `GetEvinced/stark-marketplace` after v0.1.3 closed the signed-release + binary-distribution loop. The remaining items below are all small, mechanical, high-leverage. Ship them as separate commits on `main` (this repo is allowed direct-to-main per session convention), bumping `VERSION` + `CHANGELOG.md` at the end so a single new tag (`v0.1.4`) cuts a release covering all of them.

The repo root for stark-marketplace is `/Users/aryeh/Code/Playground/stark-marketplace`. Work in the active worktree, fast-forward `main`, push. CI must pass for every push.

---

## 1. `server/` security headers (BLOCKING for prod claim)

The static origin (`server/main.go`) currently sets no app-level defenses. IAP terminates SSO but doesn't add headers. Add a header-injecting middleware that wraps the existing file handler and sets:

- `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload`
- `Content-Security-Policy: default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Permissions-Policy: camera=(), microphone=(), geolocation=()`
- `X-Frame-Options: DENY` (defense-in-depth alongside CSP frame-ancestors)

Constraints:
- Verify the SPA still loads under the CSP. The bundle is `tsc --noEmit && vite build` output. If `style-src 'self' 'unsafe-inline'` is too loose, narrow to nonces; otherwise leave inline-style allowed (Vite injects some inline styles). Test in dev by serving `WEBROOT=./web/dist` after `npm run build`.
- Add unit tests in `server/main_test.go` asserting every header is present on a 200 response AND on the IAP-redirected unauth response we serve locally (a `GET /` to a fixture WEBROOT).
- Don't add headers in IAP responses outside our control — only on responses we generate.

## 2. Web CI tightening

`.github/workflows/ci.yml` `web` job currently runs only `npm ci && npm run build`. Add:

- `npm run typecheck` (alias for `tsc --noEmit`)
- `npm run lint`
- `npm test` (vitest)

All blocking. Also add a `gofmt -l . | tee /dev/stderr | (! read)` step to the engine job to catch unformatted Go before merge (`go vet` doesn't catch formatting).

## 3. GH Actions Node 24 bump

Deprecation warnings landed on every workflow run. Bump every action that supports Node 24:
- `actions/checkout` → latest (v4 supports Node 24 in newer minor releases; pin to a specific commit SHA for supply-chain hygiene)
- `actions/setup-go` → latest
- `actions/upload-artifact` → latest
- `actions/setup-node` → latest
- `goreleaser/goreleaser-action` → latest that supports Node 24
- `sigstore/cosign-installer` → latest
- `rhysd/actionlint` → latest

Cross-check by running each workflow once on a throwaway branch; ensure no functional drift.

## 4. Investigate the `stark build` dirty-tree mystery

`stark build` on Linux CI runners produces 19 files showing as `D` (deleted) in `git status` after the remove-then-write in `internal/build/build.go`. Locally on macOS, `git status` is clean after the same command. `build --check` is clean in both environments. `git checkout -- .` did NOT restore the deletions on Linux. We worked around with goreleaser `--skip=validate` in v0.1.3, but the root cause is unknown and will bite again.

Investigation steps:
1. Add a diagnostic CI job that, on a tag-cut sign-manifest run, runs `git status --porcelain -uall` immediately after `stark build` and dumps the output to the workflow log. Compare against `git diff --stat HEAD` to see if these are working-tree-only or also index-staged.
2. Check whether `build.Write`'s `os.RemoveAll(filepath.Join(repoRoot, "dist/claude"))` survives on Linux when the file has different perms than expected.
3. Check whether file modes drift (Linux 0o644 vs HEAD mode). Run `git diff` not `git status` — if the only diff is mode, the file content matches but git still flags it.
4. Hypothesis: the line endings or final newline differ. `build.Write` does CRLF→LF normalization (`bytes.ReplaceAll`). Check whether HEAD's committed dist/claude files have trailing newlines that the rebuilt set drops, or vice versa.

When the cause is found, fix `internal/build/build.go` to match HEAD exactly, drop `--skip=validate` from sign-manifest.yml, and add a `TestWriteIsByteIdenticalToHead` integration test that fails CI if the rebuild differs from the committed tree.

## 5. Rollback runbook

Create `docs/operations/rollback.md` (new directory) covering:

- **Cloud Run revision rollback**: `gcloud run services update-traffic stark-marketplace --to-revisions=<prev>=100 --region=us-east1` — include the IAP/LB caveat that traffic flips need a min-instances revision first if cold-starting.
- **Bad-bundle yank**: how to take a published bundle out of circulation without breaking version-bump immutability. The pattern is to publish a new version of the bundle that's effectively empty/deprecated, NOT to delete the released bytes. Document the marketplace.json registry-side approach.
- **Signed-release revocation**: how to mark a signed release as poisoned. Cosign doesn't have native revocation; document the policy: bump VERSION, ship a new signed release, post an advisory in `SECURITY.md` listing the SHAs that should not be installed.
- **Who pages whom**: link to the email notification channel that `infra-ai-platform`'s uptime + 5xx alerts route to.

## 6. MCP allowlist auto-surfaced

Generate `docs/allowlist.md` deterministically from `engine/internal/validate/allowlist.go` + `engine/internal/validate/toolsallow.go`, included in the standard `stark build` flow so it stays current. Implementation:

- Add `stark allowlist --print` subcommand that emits a Markdown table of `command` allowlist + `agent.tools` allowlist.
- Have `build.Write` also write `docs/allowlist.md` as part of generated outputs.
- Add `docs/allowlist.md` to `generatedRoots` (or document why it shouldn't be).
- Drift check picks it up automatically.

## 7. Surfaced cleanup

- Annotate `v0.1.0`, `v0.1.1`, `v0.1.2` releases with a "superseded by v0.1.3 — do not use" note. Don't delete the tags or releases (destructive — they're referenced by the manifests already cosign-signed and any external consumer could pin them).
- Trigger a manual `web-deploy.yml` run so the live SPA at `marketplace.evinced.rocks` picks up the `ProvenanceBadge` shipped in 0.1.0. Verify in the browser that the badge renders.
- Verify in GCP Monitoring that the new `stark-marketplace uptime` and `stark-marketplace 5xx from LB backend` policies are green / active. If the uptime probe is flapping, debug — the `STATUS_CLASS_3XX` acceptance may need adjusting for IAP's exact response.

## Wrap-up

After all 7 land:
- Bump `VERSION` to `0.1.4` in a single final commit
- Add CHANGELOG entry summarizing items 1–6 grouped (Added / Changed / Fixed / Docs)
- Push to main; sign-manifest.yml cuts v0.1.4 with full signed-manifest + binaries
- Verify v0.1.4 release page shows all expected assets
- Report a summary of what changed, what didn't, and any new gaps surfaced.
