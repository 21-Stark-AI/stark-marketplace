# stark-marketplace — Web registry hosting (gated-static behind SSO)

> **Provisioned.** Host = `marketplace.evinced.rocks`, served on **Cloud Run + IAP**
> behind the shared platform LB. Infra primitives (SAs/IAM/GAR/NEG/IAP/host-rule/DNS)
> live in Terraform (`infra-ai-platform/infra/stark-marketplace.tf`) — no ad-hoc
> `gcloud`. The **Cloud Run service itself is deployed by this repo's CI** via WIF
> (`.github/workflows/web-deploy.yml`), per the infra-ai-platform ownership split.
> See **Provisioned implementation** at the bottom for the concrete wiring.

## Pattern: identity-aware proxy in front of static content

- **Origin:** the atomic content-hashed `web/dist` bundle (SPA shell + hashed assets +
  `index.json` + `bundles/*.json`), produced by `.github/workflows/web-deploy.yml`.
- **Gate:** an **identity-aware proxy enforcing Google Workspace SSO** sits in front of
  the origin. Options that satisfy the spec:
  - **GCP Cloud Run + IAP** (Cloud Run serves the static bundle via a tiny static
    file server image; IAP enforces the Evinced Workspace org). Recommended.
  - Equivalent: GCLB + IAP in front of a GCS backend bucket.
- **Critical invariant (spec §10):** the proxy gates **ALL data files**, not just HTML —
  `index.json`, every `bundles/<name>.json`, and any served Claude tree are behind the
  same SSO check. There is **no anonymous origin** and **no app-level user store**;
  identity is the proxy's job.

## Routing (no server rewrite needed)

The SPA uses **hash routing** (`HashRouter`): every route lives under `/#/…` on the single root
`index.html`, with assets referenced by a relative base. A deep link like
`…/#/bundle/stark-gh` therefore loads the root document and its hashed assets directly — the
dumb static origin needs **no SPA-fallback rewrite rule**, and a shared/refreshed deep link
never 404s its assets. (If you ever switch to history routing, the origin MUST rewrite unknown
paths to `/index.html` and serve a root-anchored asset base — call that out here first.)

## Caching (atomic unit)

- `assets/*` (content-hashed): `Cache-Control: public, max-age=31536000, immutable`.
- `index.html`, `index.json`, `bundles/*.json`: `Cache-Control: no-cache` (the
  cache-busted pointers). Because assets are hashed, a deploy flips atomically — a client
  never pairs a new `index.html` with a stale asset, nor an old SPA with a new index.

## Auth for SPA fetches

The SPA calls `fetch(..., { credentials: 'same-origin' })`; the proxy session cookie
authorizes the data fetches. On a 401/expired session the data layer returns a
**degraded** result (it never throws) and the UI points the user to the GitHub source
and to re-authenticate (`src/data/registry.ts`).

## Schema skew safety net

If the index `schemaVersion` is newer than the deployed SPA understands (or unreadable),
the SPA shows the degraded view rather than blank-failing (`src/data/schema.ts`,
`src/pages/DegradedPage.tsx`). N-1 schema versions still render.

## Provisioned implementation

```
browser → marketplace.evinced.rocks
        → platform HTTPS LB (host rule, *.evinced.rocks wildcard cert)
        → IAP  (Evinced Google SSO)
        → serverless NEG → Cloud Run `stark-marketplace` (server/)
```

| Piece | Owner | What |
|-------|-------|------|
| `server/` | this repo | stdlib Go static fileserver (SPA + `index.json` + `bundles/`, cache headers, `/healthz`) |
| `Dockerfile` | this repo | `web/dist` (built in CI) + the Go binary → distroless image |
| `.github/workflows/web-deploy.yml` | this repo | build → push to GAR → `gcloud run deploy` (WIF, keyless) |
| runtime SA, CI SA + WIF, GAR, NEG, IAP backend, host rule, DNS, registry slot 16 | `infra-ai-platform` (`infra/stark-marketplace.tf`) | all infra primitives |

**Cloud Run posture:** `--ingress internal-and-cloud-load-balancing` blocks the
public `*.run.app` URL (only the external LB reaches the service). The LB →
Cloud Run hop on a serverless NEG carries no Cloud Run IAM token, so the service
needs an `allUsers` `run.invoker` binding — **owned by Terraform** (gated), so the
deploy SA (`run.developer`) never calls `setIamPolicy`. Safe because ingress
restricts the path to the LB and **IAP gates every user at the edge**.

**Required repo variables** (Settings → Secrets and variables → Actions → Variables),
set from the Terraform outputs after the infra PR applies:

| Variable | Terraform output |
|----------|------------------|
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | `wif_provider_name` |
| `GCP_DEPLOY_SA` | `marketplace_ci_sa_email` |

Project (`infra-ai-platform`), region (`us-east1`), GAR repo, service name, and
runtime SA are fixed in the workflow `env:` block.

**Bootstrap order (first time only):**
1. infra-ai-platform PR → review → CI apply (creates identity, GAR, IAP backend, host rule, DNS).
2. Set the two repo variables from the TF outputs.
3. Merge this repo to `main` → CI builds + deploys the Cloud Run service.
4. Visit `https://marketplace.evinced.rocks` → IAP SSO → registry.

Until step 3's first deploy lands, the host resolves and IAP prompts, but the
backend has no Cloud Run revision yet (502/404 behind IAP) — expected.
