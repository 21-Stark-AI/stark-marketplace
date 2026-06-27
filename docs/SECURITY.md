# stark-marketplace — Security & Governance

This is a **code-distribution system**, not just config. Every artifact body is
instruction text injected into a developer's agent, and every `mcp/` entry is a
command spawned on the developer's machine. The controls below treat those as the
highest-trust surfaces (design spec §7.4, §7.5, §11, §14).

## 1. Trust model

Integrity rests on **three things together**, not on self-computed digests:

1. **Protected, linear `main`** — no force-push, no admin bypass, no deletions.
2. **A CI-signed build manifest** — produced on merge by `sign-manifest.yml` via
   GitHub OIDC → sigstore/cosign **keyless** (Fulcio cert + Rekor transparency log).
   The signer identity is `repo:21-Stark-AI/stark-marketplace` on the `main` ref.
3. **The commit SHA** — installs may pin it; the manifest binds digests to that SHA.

`stark verify-manifest` checks the cosign signature, the signer identity/issuer, and
that each recorded digest matches the committed bytes. **Self-computed digests alone
are only an anti-drift / consistency signal** — they prove the tree matches itself,
never that it is the official build. (spec §7.5 / red-team C1.)

### Where to get the signed manifest

`sign-manifest.yml` attaches `build-manifest.json`, `build-manifest.json.sig`,
`build-manifest.json.pem`, and `build-manifest.sha256` to the GitHub Release it
creates when `VERSION` changes (tag `v<VERSION>`).

End-to-end client verify:

```bash
# 1. Pull the signed bundle for a specific release (private repo → gh auth)
gh release download v0.1.0 \
  --repo 21-Stark-AI/stark-marketplace \
  --pattern 'build-manifest.json*'

# 2. Verify signature (cosign keyless) + content digests against the local checkout
stark verify-manifest --root . build-manifest.json
```

A non-zero exit code means EITHER the signature failed (wrong signer, no Rekor
entry) OR a committed file's bytes drifted from the manifest digest. Treat both
as install blockers.

## 2. Command-allowlist governance

MCP `command` values must be on the positive allowlist in
`engine/internal/validate/allowlist.go`; `agent.tools` against the allowlist in
`engine/internal/validate/toolsallow.go`. Every entry in `allowlist.go` widens the set of
binaries an MCP server may spawn on a developer's machine, so additions are explicitly
gated (spec §15.4): both files have a dedicated, last-match-wins **CODEOWNERS** entry
(`@21-Stark-AI/stark-maintainers @aryeh-stark`) on top of the `engine/**` rule. To add an
entry:

- Open a PR touching only the allowlist file with a one-paragraph justification
  (what the binary/tool does, why it is needed, who maintains it).
- Requires **maintainer approval** (`@21-Stark-AI/stark-maintainers`) **and**
  `@aryeh-stark` — CODEOWNERS marks both required on
  `engine/internal/validate/allowlist.go` and `engine/internal/validate/toolsallow.go`.
- Keep the list minimal; prefer pinned, well-known binaries (`node`, `uvx`) and
  first-party `stark-*-mcp` servers over ad-hoc tools.

## 3. Review requirements (CODEOWNERS)

| Path | Required reviewers | Min approvals |
|------|--------------------|---------------|
| `catalog/**/skills/**`, `catalog/**/commands/**`, `catalog/**/agents/**` (bodies) | maintainer **+ second reviewer** | **2** |
| `**/mcp/**` (code execution) | maintainer + reviewer **+ Aryeh** | **2** |
| `engine/internal/validate/allowlist.go`, `engine/internal/validate/toolsallow.go` (command/tool allowlists) | maintainer + Aryeh | 2 |
| `engine/**`, `schema/**`, `dist/claude/**`, `index.json`, `bundles/**` | maintainer + Aryeh | 2 |
| `.github/workflows/**`, `CODEOWNERS`, `.gitleaks.toml`, this file | maintainer + Aryeh | 2 |

**Two approvals, not one:** a CODEOWNERS entry only guarantees *who* must review; it does
not raise the *count*. The high-trust body and `**/mcp/**` paths require **TWO** distinct
approvals — the CODEOWNERS reviewer requirement **plus** repo-wide
`required_approving_review_count = 2` (set in §5). One CODEOWNERS reviewer alone would still
merge on a single approval, which is insufficient for instruction-text/code-exec surfaces.
The count is repo-wide (GitHub has no per-path count), so every PR clears 2 approvals; the
strictest path governs.

Prerequisite: the org teams `@21-Stark-AI/stark-maintainers` and
`@21-Stark-AI/stark-reviewers` must exist with write access for CODEOWNERS to bind.

## 4. CI gates (required, non-bypassable)

`ci.yml` runs on every PR. **Blocking** (errors fail the job):
`stark validate`, `stark build --check` (drift — non-bypassable gate),
`stark check-bumps` (version-bump immutability — non-bypassable gate; errors when an
artifact's canonical-source digest changed without a `version` bump),
`go test ./...` (golden + determinism + integration), gitleaks, web build, actionlint.
**Non-blocking** (surfaced only): `stark lint` body scan, capability/array warnings.

## 5. Branch protection — APPLY (manual admin step)

> These commands MUTATE repo settings. Run them once as a repo admin AFTER the
> required-status contexts have appeared at least once (push a PR so the job names
> register). **Do not run as part of automated plan execution.** Replace the
> team slugs as needed.
>
> **Why `required_approving_review_count = 2`:** GitHub's review count is repo-wide —
> there is no per-path count. A CODEOWNERS entry alone only forces "review from a Code
> Owner"; it still merges on a **single** approval. The high-trust body/MCP paths
> (`catalog/**/skills/**`, `catalog/**/agents/**`, `catalog/**/commands/**`, `**/mcp/**`)
> must clear **TWO** approvals = the CODEOWNERS reviewer requirement **plus**
> `required_approving_review_count = 2`. We set the count to 2 repo-wide (the strictest
> path governs); routine engine/config PRs simply also need a second approver.

```bash
# Require the CI status checks + linear history + code-owner review + 2 approvals, no bypass.
gh api -X PUT repos/21-Stark-AI/stark-marketplace/branches/main/protection \
  --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "engine (validate + drift + tests)",
      "secret scan (catalog)",
      "web build",
      "actionlint"
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 2,
    "require_code_owner_reviews": true,
    "dismiss_stale_reviews": true
  },
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "restrictions": null
}
JSON

# Verify it took.
gh api repos/21-Stark-AI/stark-marketplace/branches/main/protection | \
  jq '{linear: .required_linear_history.enabled, force: .allow_force_pushes.enabled,
       admins: .enforce_admins.enabled, checks: .required_status_checks.contexts,
       codeowners: .required_pull_request_reviews.require_code_owner_reviews,
       approvals: .required_pull_request_reviews.required_approving_review_count}'
```

Expected verify output: `linear: true`, `force: false`, `admins: true`,
`codeowners: true`, `approvals: 2`, and the four required contexts listed.

> **Note on the `engine` required context:** the job name is
> `engine (validate + drift + tests)` regardless of the added `check-bumps` /
> `check-bumps`-blocking steps — required-status matching is by **job name**, not step.
> The `check-bumps` step (and `build --check` drift step) being non-bypassable is a
> property of the job exiting non-zero, already covered by requiring the `engine` context.

## 6. Reporting

Suspected catalog tampering or a leaked credential: open a private security advisory
on the repo and ping `@aryeh-stark`. Rotate any exposed secret immediately — values
never live in the catalog (only `secretRef` names), so rotation is in the secret store.
