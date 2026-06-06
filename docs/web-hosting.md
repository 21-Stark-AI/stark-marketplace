# stark-marketplace — Web registry hosting (gated-static behind SSO)

> **No ad-hoc provisioning.** This documents the target pattern. Provisioning lands
> through the standard Evinced IaC path (Terraform), not from this repo's CI.

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
