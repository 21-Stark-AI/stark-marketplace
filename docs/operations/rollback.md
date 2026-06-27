# stark-marketplace — Rollback runbook

This runbook covers three rollback scenarios. Read [`SECURITY.md`](../SECURITY.md) first — the trust model dictates what "rollback" means here. There is **no destructive rollback** of a signed release: signatures are immutable, transparency-logged, and possibly already pinned by consumers.

## 1. Cloud Run revision rollback (web origin)

The static origin at `marketplace.21stark.com` is a Cloud Run service.
Bad deploys are rolled back by traffic-flipping to a known-good revision — the
service itself is never deleted.

```bash
# List recent revisions
gcloud run revisions list \
  --service stark-marketplace \
  --region us-central1 \
  --project ev-infra-group \
  --limit 5

# Flip 100% of traffic to a prior revision
gcloud run services update-traffic stark-marketplace \
  --region us-central1 \
  --project ev-infra-group \
  --to-revisions=stark-marketplace-00007-xyz=100
```

**Caveats:**
- The platform LB sends traffic through a serverless NEG; the flip is propagated by the next request, no LB change needed.
- If the bad revision was deployed with `--min-instances=0` and the prior one was warm, expect ~1s cold-start on first user request after the flip.
- The `web-deploy.yml` workflow is idempotent and won't auto-deploy until the next push that touches `web/**`, `server/**`, `index.json`, `bundles/**`, or `Dockerfile`. Once the underlying issue is fixed, re-push will deploy a new revision; the rollback gets superseded automatically.

**Verify the rollback:**
- `curl -s -o /dev/null -w '%{http_code}\n' https://marketplace.21stark.com/healthz` (expect `200`).
- The `stark-marketplace uptime` and `stark-marketplace 5xx from LB backend` alert policies (`ev-infra-group/infra/stark-marketplace.tf`) should clear within one alignment window (5 min).

## 2. Bad-bundle yank (catalog)

A published bundle version is **content-locked** by `stark check-bumps`. Yanking is not "delete the bytes" — it's "publish a successor that supersedes it."

**Procedure:**
1. **Don't** edit the affected artifact in place. `check-bumps` would block the PR; even if you bypassed it, the cosign-signed manifest for the prior release still records the old digest, and any consumer with `stark verify-manifest` against the old release will still see the artifact as valid.
2. **Do** ship a new version of the bundle (or the affected artifact) that:
   - Has a higher SemVer than the bad one
   - Replaces the bad behavior with either a fixed implementation or an empty/no-op shell that prints a deprecation notice
   - References the bad version in its CHANGELOG entry
3. **Then** post an advisory:
   - Update `docs/SECURITY.md` with a "Yanked versions" section listing `bundle/version` pairs and the reason
   - Edit the affected GitHub Release page notes with a header: `⚠️ DO NOT USE — superseded by vX.Y.Z due to <one-line reason>`. Don't delete the release (consumers may have pinned the SHA; deleting breaks `stark verify-manifest`).
   - Email/Slack the alert channel (notification channel `email` in `ev-infra-group/infra/monitoring.tf`) with the same advisory.

## 3. Signed-release revocation

Cosign keyless signatures have **no native revocation**. The transparency log is append-only; we cannot un-sign. What we can do:

1. **Tag the bad release**: edit its GitHub Release notes with `⚠️ DO NOT INSTALL — see docs/SECURITY.md §<n>`. Notes are mutable; the underlying signed bytes are not.
2. **Cut a successor**: bump `VERSION`, push to main. `sign-manifest.yml` produces `v<next>` with a fresh signed manifest. This is the only artifact `stark verify-manifest` will validate against once the advisory is published.
3. **Document in `docs/SECURITY.md`**: add the bad SHA + tag + reason to a "Revoked releases" subsection. `stark verify-manifest --root .` against the bad release will still cryptographically succeed (we can't help that); the docs are the policy layer.
4. **If the bad signed manifest reveals a compromised signer identity** (e.g., the workflow ref subject was modified to allow another workflow to mint signatures), rotate the pin in `engine/internal/provenance/verify.go` `signerIdentity` and re-sign everything; existing consumers will need to upgrade `stark` to the new verifier.

The cosign signer identity is pinned exactly in `engine/internal/provenance/verify.go`:
```
https://github.com/21-Stark-AI/stark-marketplace/.github/workflows/sign-manifest.yml@refs/heads/main
```
A signature from any other workflow, ref, or repo fails `stark verify-manifest`.

## 4. Who pages whom

- **Cloud Run / LB / 5xx**: GCP Monitoring alerts route to the `email` notification channel defined in `ev-infra-group/infra/monitoring.tf` (currently the email used to bootstrap the alerting). Alerts auto-close at 30 min.
- **Catalog / signing pipeline failure**: CI failures on `sign-manifest.yml` block the release but don't page. Watch via `gh run watch` or set up a personal GitHub notification on workflow failures.
- **For incident escalation**: notify `@aryeh-stark` (CODEOWNERS-required on `engine/internal/validate/allowlist.go` and `engine/internal/provenance/`).

## 5. Drills

- Rolling back a Cloud Run revision: do this on a quiet day at least once per quarter. The command is irreversible only in the sense that the prior revision becomes the live one; flipping back is one more `update-traffic` call.
- Yanking a bundle: practice on a no-op skill (e.g., add a `stark-test-yank` bundle, "yank" it, verify the advisory flow). Don't ship the practice yank publicly.
