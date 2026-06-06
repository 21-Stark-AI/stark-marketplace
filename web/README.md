# stark-marketplace web registry (SPA)

Strict-TypeScript + Vite static SPA. Reads the lean `index.json` for faceted search and
per-bundle `bundles/<name>.json` for detail on demand. No app server for data — the index
is the API. SSO is enforced by the hosting proxy, not the app (see `../docs/web-hosting.md`).

## Develop

```bash
npm install
npm run dev          # local dev server
npm test             # vitest (unit + component + smoke)
npm run typecheck    # tsc --noEmit (strict)
npm run lint
npm run build        # tsc --noEmit && vite build → dist/ (hashed assets)
```

To run `dev`/`preview` against real data, copy a built `index.json` + `bundles/` into
`web/public/` (CI does this from the committed engine output at deploy time).

## Data contract

`src/types/registry.ts` mirrors the engine's emitted JSON (spec §7.5). Unknown fields are
ignored (forward compatible); `schemaVersion` skew degrades gracefully (`src/data/schema.ts`).

## Deploy

`.github/workflows/web-deploy.yml` builds the SPA, stages `index.json` + `bundles/` into
`dist/`, and uploads the whole thing as one atomic content-hashed unit. The publish step is
gated on the Evinced-standard hosting origin (`../docs/web-hosting.md`).
