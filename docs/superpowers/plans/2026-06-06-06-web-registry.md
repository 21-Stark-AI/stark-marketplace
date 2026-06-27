# stark-marketplace — Slice 6: Web registry (SSO-gated static SPA) Implementation Plan

> Historical note, 2026-06-23: current hosting is public at
> `https://marketplace.21stark.com` in `ev-infra-group` without IAP.
> See `docs/web-hosting.md` for live hosting. This plan preserves the original
> SSO-gated implementation context.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a strict-TypeScript + Vite static SPA under `web/` that reads the **lean `index.json`** for faceted search and per-bundle **`bundles/<name>.json`** for detail-on-demand, renders bundle/artifact detail with per-surface install instructions + native/emulated badges + dependency graph + GitHub deep links, **degrades gracefully** on `schemaVersion`/`version` skew, and deploys as one atomic content-hashed unit behind an 21 Stark AI-standard SSO identity-aware proxy.

**Architecture:** No app server for data — **the index IS the API**. The SPA fetches `index.json` once at boot (cache-busted by the SPA build hash), builds an in-memory faceted search over the lean records, and lazy-fetches `bundles/<name>.json` only when a detail route is opened. A thin `registry` data layer owns schema-version negotiation and treats every JSON read as untrusted/forward-compatible (unknown fields ignored; a `schemaVersion` outside the supported range routes to a graceful-degrade view, never a blank screen). React + React Router render search and detail; pure functions (search filter, install-snippet generation, dep-graph layout) are unit-tested with vitest; a jsdom smoke test boots the app against a fixture index. CI builds SPA + index together and an infra note documents the gated-static hosting pattern (no ad-hoc provisioning).

**Tech Stack:** TypeScript 5.x (`strict`, ESM, narrow types — no `any`), Vite 5, React 18 + react-router-dom 6, vitest + @testing-library/react + jsdom, ESLint (typescript-eslint). Package manager: `npm` (lockfile committed). All web commands run from `web/`.

**Consistency anchor:** the TS index/detail types defined in Task 3 MUST match the JSON the engine emits per CC-2/CC-3 (+ spec §7.5) and the Go `model` types in plan 01 (Task 3). The lean record mirrors the engine's index builder — top-level key **`artifacts`**, each row carries `description`, `schemaVersion` is an int. The detail mirrors `model.Bundle` + `model.Artifact` with per-runtime `support`, `requires`, `diverged`, `fidelityNotes`, and an **`outputs`** map of `{path, kind, key, sentinel, emulated}` arrays; the SPA **derives** display `outputPaths[rt]` = `outputs[rt][0].path` rather than reading a flat map. Field names below are the contract — if plan 02/03 renames a JSON field, this slice's types and fixtures change with it.

---

## A. File / directory structure

```
web/
  package.json
  package-lock.json
  tsconfig.json                         # strict, ESM, bundler resolution
  vite.config.ts                        # build + base path + vitest config
  .eslintrc.cjs
  index.html                            # SPA shell (loads /src/main.tsx)
  public/
    # index.json + bundles/ are copied here by CI at build time (Task 11);
    # a checked-in fixture lives under src/__fixtures__ for tests.
  src/
    main.tsx                            # React root + router mount (Task 8)
    types/registry.ts                   # index/detail TS types — THE shared contract (Task 3)
    data/schema.ts                      # SUPPORTED_SCHEMA + version negotiation (Task 4)
    data/registry.ts                    # fetch + parse index/detail, forward-compat (Task 5)
    search/filter.ts                    # pure faceted filter (Task 6)
    install/snippets.ts                 # per-surface install instructions (Task 7)
    graph/deps.ts                       # dependency-graph adjacency builder (Task 10)
    components/SupportBadges.tsx        # native/emulated/unsupported badges (Task 9)
    components/Facets.tsx               # facet controls (Task 8)
    components/InstallInstructions.tsx  # render snippets (Task 9)
    components/DependencyGraph.tsx      # render dep graph (Task 10)
    pages/SearchPage.tsx                # faceted search list (Task 8)
    pages/BundleDetailPage.tsx          # bundle + artifact detail (Task 9)
    pages/DegradedPage.tsx             # graceful-degrade view (Task 4)
    app.tsx                             # route table + data bootstrap (Task 8)
    __fixtures__/index.json             # lean fixture (current schemaVersion) (Task 3)
    __fixtures__/index.skewed.json      # bumped schemaVersion fixture (Task 4)
    __fixtures__/bundles/stark-review.json  # detail fixture (Task 3)
  README.md                             # build/run/deploy + hosting note (Task 12)
.github/workflows/web-deploy.yml        # atomic SPA+index build/publish (Task 11)
docs/web-hosting.md                     # gated-static SSO infra note (Task 12)
```

Every step runs from `web/` unless noted. The repo root is the parent of `web/`.

---

### Task 1: Initialize the Vite + React + TS project

**Files:**
- Create: `web/package.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/.eslintrc.cjs`

- [ ] **Step 1: Write `web/package.json`**

```json
{
  "name": "@21stark-ai/stark-marketplace-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "lint": "eslint 'src/**/*.{ts,tsx}'",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.26.2"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.5.0",
    "@testing-library/react": "^16.0.1",
    "@types/react": "^18.3.5",
    "@types/react-dom": "^18.3.0",
    "@typescript-eslint/eslint-plugin": "^8.6.0",
    "@typescript-eslint/parser": "^8.6.0",
    "@vitejs/plugin-react": "^4.3.1",
    "eslint": "^8.57.0",
    "jsdom": "^25.0.0",
    "typescript": "^5.6.2",
    "vite": "^5.4.6",
    "vitest": "^2.1.1"
  }
}
```

- [ ] **Step 2: Write `web/tsconfig.json` (strict, ESM)**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noFallthroughCasesInSwitch": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true,
    "skipLibCheck": true,
    "types": ["vitest/globals", "@testing-library/jest-dom"],
    "resolveJsonModule": true
  },
  "include": ["src", "vite.config.ts"]
}
```

- [ ] **Step 3: Write `web/vite.config.ts` (build + vitest)**

```ts
/// <reference types="vitest" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  // Served from the registry root behind the proxy; relative base keeps the
  // content-hashed bundle portable across the gated-static origin.
  base: './',
  build: {
    // Long-cache hashed assets; index.html is the cache-busted pointer.
    rollupOptions: { output: { entryFileNames: 'assets/[name].[hash].js', chunkFileNames: 'assets/[name].[hash].js', assetFileNames: 'assets/[name].[hash].[ext]' } },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
});
```

- [ ] **Step 4: Write the SPA shell, root, eslint, test setup**

`web/index.html`:
```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>stark-marketplace</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

`web/src/main.tsx`:
```tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

const el = document.getElementById('root');
if (el) {
  createRoot(el).render(
    <StrictMode>
      <div>stark-marketplace</div>
    </StrictMode>,
  );
}
```

`web/.eslintrc.cjs`:
```js
module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  parserOptions: { ecmaVersion: 2022, sourceType: 'module' },
  plugins: ['@typescript-eslint'],
  extends: ['eslint:recommended', 'plugin:@typescript-eslint/recommended'],
  env: { browser: true, es2022: true },
  rules: { '@typescript-eslint/no-explicit-any': 'error' },
  ignorePatterns: ['dist', 'node_modules'],
};
```

`web/src/test-setup.ts`:
```ts
import '@testing-library/jest-dom/vitest';
```

- [ ] **Step 5: Install and verify the toolchain**

Run from `web/`:
```bash
cd web && npm install && npm run typecheck && npm run build && cd ..
```
Expected: `npm install` writes `package-lock.json`; typecheck passes; `vite build` emits `web/dist/` with hashed `assets/*.js`.

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/package-lock.json web/tsconfig.json web/vite.config.ts web/index.html web/src/main.tsx web/.eslintrc.cjs web/src/test-setup.ts
git commit -m "feat(web): init Vite + React + strict-TS SPA scaffold"
```

---

### Task 2: Smoke vitest wiring (prove the harness runs)

**Files:**
- Test: `web/src/smoke.test.ts`

- [ ] **Step 1: Write the failing smoke test**

`web/src/smoke.test.ts`:
```ts
import { describe, it, expect } from 'vitest';
import { sum } from './smoke';

describe('vitest harness', () => {
  it('runs and imports app code', () => {
    expect(sum(2, 3)).toBe(5);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npm test -- src/smoke.test.ts`
Expected: FAIL — cannot resolve `./smoke` (module does not exist).

- [ ] **Step 3: Implement the trivial module**

`web/src/smoke.ts`:
```ts
export const sum = (a: number, b: number): number => a + b;
```

- [ ] **Step 4: Run to verify pass**

Run: `cd web && npm test -- src/smoke.test.ts`
Expected: PASS (1 test).

- [ ] **Step 5: Commit**

```bash
git add web/src/smoke.ts web/src/smoke.test.ts
git commit -m "test(web): vitest harness smoke test"
```

---

### Task 3: Registry TS types + fixtures (the shared contract)

**Files:**
- Create: `web/src/types/registry.ts`
- Create: `web/src/__fixtures__/index.json`
- Create: `web/src/__fixtures__/bundles/stark-review.json`
- Test: `web/src/types/registry.test.ts`

> These types MUST match the engine's emitted JSON (CC-2/CC-3 + spec §7.5) and plan 01's Go `model` types. Lean record = index-builder output (top-level key **`artifacts`**, each row carries `description` for full-text search; `schemaVersion` is an int). Detail = `Bundle`+`Artifact` with per-runtime `support`, `requires`, `diverged`, `fidelityNotes`, and an **`outputs`** map of `{path, kind, key, sentinel, emulated}` arrays. The SPA derives the display `outputPaths[rt]` = `outputs[rt][0].path` rather than expecting a flat `outputPaths` from the engine. Consumers ignore unknown fields, so types model only the fields the SPA reads.
>
> **Fixtures are engine-shaped.** Ideally these `__fixtures__` are generated from `stark build` output (copy the real `index.json` + `bundles/stark-review.json`); at minimum they must be kept byte-aligned to the engine emission and asserted by the type-guard tests. If plan 02/03 renames or reshapes a JSON field, regenerate the fixtures, do not hand-shape them per consumer.

- [ ] **Step 1: Write the failing type-guard test**

`web/src/types/registry.test.ts`:
```ts
import { describe, it, expect } from 'vitest';
import indexFixture from '../__fixtures__/index.json';
import detailFixture from '../__fixtures__/bundles/stark-review.json';
import { isLeanIndex, isBundleDetail } from './registry';

describe('registry type guards', () => {
  it('accepts the lean index fixture', () => {
    expect(isLeanIndex(indexFixture)).toBe(true);
  });
  it('accepts the bundle detail fixture', () => {
    expect(isBundleDetail(detailFixture)).toBe(true);
  });
  it('ignores unknown extra fields (forward compat)', () => {
    const withExtra = { ...(indexFixture as object), futureField: 42 };
    expect(isLeanIndex(withExtra)).toBe(true);
  });
  it('rejects a record missing schemaVersion', () => {
    expect(isLeanIndex({ artifacts: [] })).toBe(false);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npm test -- src/types/registry.test.ts`
Expected: FAIL — cannot resolve `./registry` / fixtures.

- [ ] **Step 3: Write the fixtures**

`web/src/__fixtures__/index.json` (lean; matches spec §7.5 lean shape):
```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-06-06T00:00:00Z",
  "artifacts": [
    {
      "name": "stark-review",
      "type": "skill",
      "bundle": "stark-review",
      "description": "Single-agent PR review with triage-selected domains.",
      "tags": ["pr", "review", "multi-agent"],
      "category": "code-review",
      "maturity": "stable",
      "version": "0.7.0",
      "digest": "sha256:aaa",
      "support": { "claude": "native", "codex": "native", "gemini": "emulated" }
    },
    {
      "name": "review",
      "type": "command",
      "bundle": "stark-review",
      "description": "Run a PR review.",
      "tags": ["pr", "review"],
      "category": "code-review",
      "maturity": "stable",
      "version": "0.7.0",
      "digest": "sha256:bbb",
      "support": { "claude": "native", "codex": "native", "gemini": "native" }
    },
    {
      "name": "gh",
      "type": "mcp",
      "bundle": "stark-gh",
      "description": "GitHub MCP server.",
      "tags": ["github"],
      "category": "vcs",
      "maturity": "beta",
      "version": "1.2.0",
      "digest": "sha256:ccc",
      "support": { "claude": "native", "codex": "native", "gemini": "native" }
    }
  ]
}
```

`web/src/__fixtures__/bundles/stark-review.json` (detail; matches §7.5 detail shape):
```json
{
  "schemaVersion": 1,
  "bundle": {
    "name": "stark-review",
    "version": "0.7.0",
    "description": "Multi-agent PR review toolkit.",
    "category": "code-review",
    "tags": ["pr", "review"],
    "maturity": "stable",
    "owner": { "name": "21 Stark AI", "email": "engineering@21stark.com" },
    "runtimes": ["claude", "codex", "gemini"],
    "homepage": "https://github.com/21-Stark-AI/stark-marketplace/tree/main/catalog/stark-review"
  },
  "artifacts": [
    {
      "name": "stark-review",
      "type": "skill",
      "description": "Single-agent PR review with triage-selected domains.",
      "version": "0.7.0",
      "tags": ["pr", "review", "multi-agent"],
      "maturity": "stable",
      "requires": [{ "type": "skill", "ref": "stark-session" }],
      "support": { "claude": "native", "codex": "native", "gemini": "emulated" },
      "diverged": false,
      "outputs": {
        "claude": [
          { "path": "skills/stark-review/SKILL.md", "kind": "file", "key": null, "sentinel": null, "emulated": false }
        ],
        "codex": [
          { "path": ".agents/skills/stark-review/SKILL.md", "kind": "file", "key": null, "sentinel": null, "emulated": false }
        ],
        "gemini": [
          { "path": "GEMINI.md", "kind": "sentinel", "key": null, "sentinel": "stark-review", "emulated": true }
        ]
      },
      "fidelityNotes": {
        "gemini": "EMULATED — derived shape; may not auto-activate; verify."
      },
      "sourcePath": "catalog/stark-review/skills/stark-review.md"
    },
    {
      "name": "review",
      "type": "command",
      "description": "Run a PR review.",
      "version": "0.7.0",
      "tags": ["pr", "review"],
      "maturity": "stable",
      "requires": [],
      "support": { "claude": "native", "codex": "native", "gemini": "native" },
      "diverged": false,
      "outputs": {
        "claude": [
          { "path": "commands/review.md", "kind": "file", "key": null, "sentinel": null, "emulated": false }
        ],
        "codex": [
          { "path": ".agents/skills/review/SKILL.md", "kind": "file", "key": null, "sentinel": null, "emulated": false }
        ],
        "gemini": [
          { "path": ".gemini/commands/review.toml", "kind": "file", "key": null, "sentinel": null, "emulated": false }
        ]
      },
      "fidelityNotes": {},
      "sourcePath": "catalog/stark-review/commands/review.md"
    }
  ],
  "dependencyClosure": [
    { "from": "stark-review/stark-review", "to": "stark-review/stark-session" }
  ]
}
```

- [ ] **Step 4: Implement the types + guards**

`web/src/types/registry.ts`:
```ts
// Mirrors the engine's emitted JSON (spec §7.5) and the Go model (plan 01 Task 3).
// Consumers IGNORE unknown fields — these interfaces model only what the SPA reads.

export type Runtime = 'claude' | 'codex' | 'gemini';
export type ArtifactType = 'skill' | 'prompt' | 'command' | 'agent' | 'mcp';
export type Maturity = 'experimental' | 'beta' | 'stable' | 'deprecated';
export type SupportLevel = 'native' | 'emulated' | 'unsupported';

export type SupportMatrix = Partial<Record<Runtime, SupportLevel>>;

/** One row of the lean index.json — only search-facing fields (spec §7.5). */
export interface LeanArtifact {
  readonly name: string;
  readonly type: ArtifactType;
  readonly bundle: string;
  readonly description: string;
  readonly tags: readonly string[];
  readonly category: string;
  readonly maturity: Maturity;
  readonly version: string;
  readonly digest: string;
  readonly support: SupportMatrix;
}

/** Top-level lean index.json document. */
export interface LeanIndex {
  readonly schemaVersion: number;
  readonly generatedAt?: string;
  readonly artifacts: readonly LeanArtifact[];
}

export interface Requirement {
  readonly type: ArtifactType;
  readonly ref: string; // "name" (same bundle) or "bundle/name"
}

export interface BundleMeta {
  readonly name: string;
  readonly version: string;
  readonly description: string;
  readonly category: string;
  readonly tags: readonly string[];
  readonly maturity: Maturity;
  readonly owner: { readonly name: string; readonly email?: string };
  readonly runtimes: readonly Runtime[];
  readonly homepage?: string;
}

export type OutputKind = 'file' | 'mergeJSONKey' | 'mergeTOMLKey' | 'sentinel';

/** One engine-emitted output for a (artifact, runtime). Mirrors CC-3 `outputs[]`. */
export interface ArtifactOutput {
  readonly path: string;
  readonly kind: OutputKind;
  readonly key: string | null;       // merge target key for merge* kinds
  readonly sentinel: string | null;  // sentinel name for sentinel kind
  readonly emulated: boolean;
}

export type OutputMatrix = Partial<Record<Runtime, readonly ArtifactOutput[]>>;

export interface DetailArtifact {
  readonly name: string;
  readonly type: ArtifactType;
  readonly description: string;
  readonly version: string;
  readonly tags: readonly string[];
  readonly maturity: Maturity;
  readonly requires: readonly Requirement[];
  readonly support: SupportMatrix;
  readonly diverged: boolean;
  // Engine-emitted per-runtime outputs (CC-3). Display paths are DERIVED, not read flat.
  readonly outputs: OutputMatrix;
  readonly fidelityNotes: Partial<Record<Runtime, string>>;
  readonly sourcePath: string;
}

/** Derive the display path for a runtime = first emitted output's path (CC-3). */
export function outputPathFor(a: DetailArtifact, rt: Runtime): string | undefined {
  return a.outputs[rt]?.[0]?.path;
}

export interface DependencyEdge {
  readonly from: string;
  readonly to: string;
}

/** Per-bundle bundles/<name>.json document. */
export interface BundleDetail {
  readonly schemaVersion: number;
  readonly bundle: BundleMeta;
  readonly artifacts: readonly DetailArtifact[];
  readonly dependencyClosure: readonly DependencyEdge[];
}

const isRecord = (v: unknown): v is Record<string, unknown> =>
  typeof v === 'object' && v !== null;

export function isLeanIndex(v: unknown): v is LeanIndex {
  return (
    isRecord(v) &&
    typeof v.schemaVersion === 'number' &&
    Array.isArray(v.artifacts)
  );
}

export function isBundleDetail(v: unknown): v is BundleDetail {
  return (
    isRecord(v) &&
    typeof v.schemaVersion === 'number' &&
    isRecord(v.bundle) &&
    Array.isArray(v.artifacts)
  );
}
```

- [ ] **Step 5: Run to verify pass**

Run: `cd web && npm test -- src/types/registry.test.ts && npm run typecheck`
Expected: PASS (4 tests); typecheck clean.

- [ ] **Step 6: Commit**

```bash
git add web/src/types/ web/src/__fixtures__/index.json web/src/__fixtures__/bundles/stark-review.json
git commit -m "feat(web): registry index/detail types + fixtures matching engine JSON"
```

---

### Task 4: Schema-version negotiation (graceful degrade core)

**Files:**
- Create: `web/src/data/schema.ts`
- Create: `web/src/__fixtures__/index.skewed.json`
- Test: `web/src/data/schema.test.ts`

- [ ] **Step 1: Write the skewed fixture**

`web/src/__fixtures__/index.skewed.json` (bumped schemaVersion = a genuine break):
```json
{ "schemaVersion": 99, "artifacts": [] }
```

- [ ] **Step 2: Write failing tests**

`web/src/data/schema.test.ts`:
```ts
import { describe, it, expect } from 'vitest';
import skewed from '../__fixtures__/index.skewed.json';
import current from '../__fixtures__/index.json';
import { SUPPORTED_SCHEMA, negotiate } from './schema';

describe('schema negotiation', () => {
  it('accepts the current supported schemaVersion', () => {
    const r = negotiate((current as { schemaVersion: number }).schemaVersion);
    expect(r.ok).toBe(true);
  });
  it('accepts N-1 (older index, newer SPA)', () => {
    const r = negotiate(SUPPORTED_SCHEMA - 1);
    expect(r.ok).toBe(true);
  });
  it('degrades on a future schemaVersion (newer index, older SPA)', () => {
    const r = negotiate((skewed as { schemaVersion: number }).schemaVersion);
    expect(r.ok).toBe(false);
    if (!r.ok) {
      expect(r.reason).toBe('unsupported-newer');
      expect(r.indexVersion).toBe(99);
    }
  });
  it('degrades on a missing/NaN version', () => {
    const r = negotiate(Number.NaN);
    expect(r.ok).toBe(false);
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd web && npm test -- src/data/schema.test.ts`
Expected: FAIL — cannot resolve `./schema`.

- [ ] **Step 4: Implement**

`web/src/data/schema.ts`:
```ts
// The schemaVersion this SPA build understands. Bumped only on genuine breaks.
// Backward compat by convention: additive fields within a version (unknown fields
// ignored); older indexes down to SUPPORTED_SCHEMA - MIN_BACK still render (N-1).
export const SUPPORTED_SCHEMA = 1;
export const MIN_BACK = 1; // accept versions [SUPPORTED_SCHEMA - MIN_BACK, SUPPORTED_SCHEMA]

export type Negotiation =
  | { readonly ok: true; readonly indexVersion: number }
  | {
      readonly ok: false;
      readonly reason: 'unsupported-newer' | 'unsupported-older' | 'invalid';
      readonly indexVersion: number;
    };

export function negotiate(indexVersion: number): Negotiation {
  if (!Number.isInteger(indexVersion)) {
    return { ok: false, reason: 'invalid', indexVersion };
  }
  if (indexVersion > SUPPORTED_SCHEMA) {
    return { ok: false, reason: 'unsupported-newer', indexVersion };
  }
  if (indexVersion < SUPPORTED_SCHEMA - MIN_BACK) {
    return { ok: false, reason: 'unsupported-older', indexVersion };
  }
  return { ok: true, indexVersion };
}
```

- [ ] **Step 5: Run to verify pass**

Run: `cd web && npm test -- src/data/schema.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add web/src/data/schema.ts web/src/__fixtures__/index.skewed.json web/src/data/schema.test.ts
git commit -m "feat(web): schemaVersion negotiation (N-1 support + graceful-degrade signal)"
```

---

### Task 5: Registry data layer (fetch + parse, forward-compatible)

**Files:**
- Create: `web/src/data/registry.ts`
- Test: `web/src/data/registry.test.ts`

- [ ] **Step 1: Write failing tests (mock fetch)**

`web/src/data/registry.test.ts`:
```ts
import { describe, it, expect, vi, afterEach } from 'vitest';
import indexFixture from '../__fixtures__/index.json';
import detailFixture from '../__fixtures__/bundles/stark-review.json';
import skewed from '../__fixtures__/index.skewed.json';
import { loadIndex, loadBundleDetail, registryError } from './registry';

const mockFetch = (status: number, body: unknown) =>
  vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response);

afterEach(() => vi.restoreAllMocks());

describe('loadIndex', () => {
  it('returns a parsed lean index on a supported version', async () => {
    vi.stubGlobal('fetch', mockFetch(200, indexFixture));
    const res = await loadIndex('/index.json');
    expect(res.kind).toBe('ok');
    if (res.kind === 'ok') expect(res.index.artifacts.length).toBe(3);
  });

  it('degrades (not throws) on a skewed schemaVersion', async () => {
    vi.stubGlobal('fetch', mockFetch(200, skewed));
    const res = await loadIndex('/index.json');
    expect(res.kind).toBe('degraded');
    if (res.kind === 'degraded') expect(res.reason).toBe('unsupported-newer');
  });

  it('degrades on a non-conforming payload', async () => {
    vi.stubGlobal('fetch', mockFetch(200, { nope: true }));
    const res = await loadIndex('/index.json');
    expect(res.kind).toBe('degraded');
  });

  it('degrades on an HTTP error (e.g. proxy 401/5xx)', async () => {
    vi.stubGlobal('fetch', mockFetch(503, null));
    const res = await loadIndex('/index.json');
    expect(res.kind).toBe('degraded');
    if (res.kind === 'degraded') expect(res.reason).toBe('fetch-failed');
  });
});

describe('loadBundleDetail', () => {
  it('returns parsed detail', async () => {
    vi.stubGlobal('fetch', mockFetch(200, detailFixture));
    const res = await loadBundleDetail('stark-review');
    expect(res.kind).toBe('ok');
    if (res.kind === 'ok') expect(res.detail.bundle.name).toBe('stark-review');
  });
});

describe('registryError', () => {
  it('maps reasons to user-facing copy mentioning the GitHub source', () => {
    expect(registryError('unsupported-newer')).toMatch(/github/i);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npm test -- src/data/registry.test.ts`
Expected: FAIL — cannot resolve `./registry`.

- [ ] **Step 3: Implement**

`web/src/data/registry.ts`:
```ts
import {
  isLeanIndex,
  isBundleDetail,
  type LeanIndex,
  type BundleDetail,
} from '../types/registry';
import { negotiate, type Negotiation } from './schema';

const GITHUB_SOURCE = 'https://github.com/21-Stark-AI/stark-marketplace';

export type DegradeReason =
  | Exclude<Extract<Negotiation, { ok: false }>['reason'], never>
  | 'fetch-failed'
  | 'malformed';

export type IndexResult =
  | { readonly kind: 'ok'; readonly index: LeanIndex }
  | { readonly kind: 'degraded'; readonly reason: DegradeReason; readonly githubUrl: string };

export type DetailResult =
  | { readonly kind: 'ok'; readonly detail: BundleDetail }
  | { readonly kind: 'degraded'; readonly reason: DegradeReason; readonly githubUrl: string };

const degradedIndex = (reason: DegradeReason): IndexResult => ({
  kind: 'degraded',
  reason,
  githubUrl: GITHUB_SOURCE,
});

async function fetchJson(url: string): Promise<unknown | undefined> {
  try {
    const resp = await fetch(url, { credentials: 'same-origin' });
    if (!resp.ok) return undefined;
    return (await resp.json()) as unknown;
  } catch {
    return undefined;
  }
}

export async function loadIndex(url = './index.json'): Promise<IndexResult> {
  const raw = await fetchJson(url);
  if (raw === undefined) return degradedIndex('fetch-failed');
  if (!isLeanIndex(raw)) return degradedIndex('malformed');
  const neg = negotiate(raw.schemaVersion);
  if (!neg.ok) return degradedIndex(neg.reason);
  return { kind: 'ok', index: raw };
}

export async function loadBundleDetail(bundle: string): Promise<DetailResult> {
  const url = `./bundles/${encodeURIComponent(bundle)}.json`;
  const raw = await fetchJson(url);
  if (raw === undefined) return { kind: 'degraded', reason: 'fetch-failed', githubUrl: GITHUB_SOURCE };
  if (!isBundleDetail(raw)) return { kind: 'degraded', reason: 'malformed', githubUrl: GITHUB_SOURCE };
  const neg = negotiate(raw.schemaVersion);
  if (!neg.ok) return { kind: 'degraded', reason: neg.reason, githubUrl: GITHUB_SOURCE };
  return { kind: 'ok', detail: raw };
}

export function registryError(reason: DegradeReason): string {
  switch (reason) {
    case 'unsupported-newer':
      return `The registry index is newer than this app build. The registry is updating — meanwhile, browse the source on GitHub: ${GITHUB_SOURCE}`;
    case 'unsupported-older':
      return `This app build expects a newer registry index. Browse the source on GitHub: ${GITHUB_SOURCE}`;
    case 'fetch-failed':
      return `Couldn't reach the registry data (you may need to re-authenticate). Source on GitHub: ${GITHUB_SOURCE}`;
    case 'malformed':
    case 'invalid':
      return `The registry index couldn't be read. Browse the source on GitHub: ${GITHUB_SOURCE}`;
  }
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd web && npm test -- src/data/registry.test.ts && npm run typecheck`
Expected: PASS (all tests); typecheck clean (note `DegradeReason` includes `'invalid'` via the negotiation reason union).

- [ ] **Step 5: Commit**

```bash
git add web/src/data/registry.ts web/src/data/registry.test.ts
git commit -m "feat(web): registry data layer — fetch/parse with graceful degrade, never throws"
```

---

### Task 6: Faceted search (pure filter)

**Files:**
- Create: `web/src/search/filter.ts`
- Test: `web/src/search/filter.test.ts`

- [ ] **Step 1: Write failing tests**

`web/src/search/filter.test.ts`:
```ts
import { describe, it, expect } from 'vitest';
import indexFixture from '../__fixtures__/index.json';
import { isLeanIndex } from '../types/registry';
import { filterArtifacts, collectFacets, type Facets } from './filter';

const index = (() => {
  if (!isLeanIndex(indexFixture)) throw new Error('bad fixture');
  return indexFixture;
})();

const empty: Facets = { query: '', type: undefined, tag: undefined, category: undefined, runtime: undefined, maturity: undefined };

describe('filterArtifacts', () => {
  it('returns all with empty facets (deprecated already excluded by default)', () => {
    expect(filterArtifacts(index.artifacts, empty).length).toBe(3);
  });
  it('filters by type', () => {
    const r = filterArtifacts(index.artifacts, { ...empty, type: 'mcp' });
    expect(r.map((a) => a.name)).toEqual(['gh']);
  });
  it('filters by tag', () => {
    const r = filterArtifacts(index.artifacts, { ...empty, tag: 'review' });
    expect(r.map((a) => a.name).sort()).toEqual(['review', 'stark-review']);
  });
  it('filters by runtime support (native or emulated counts as supported)', () => {
    const r = filterArtifacts(index.artifacts, { ...empty, runtime: 'gemini' });
    expect(r.length).toBe(3);
  });
  it('full-text matches name + description', () => {
    const r = filterArtifacts(index.artifacts, { ...empty, query: 'github' });
    expect(r.map((a) => a.name)).toEqual(['gh']);
  });
  it('combines facets (AND)', () => {
    const r = filterArtifacts(index.artifacts, { ...empty, type: 'command', tag: 'review' });
    expect(r.map((a) => a.name)).toEqual(['review']);
  });
});

describe('collectFacets', () => {
  it('returns sorted unique facet values', () => {
    const f = collectFacets(index.artifacts);
    expect(f.types).toEqual(['command', 'mcp', 'skill']);
    expect(f.tags).toContain('review');
    expect(f.runtimes).toEqual(['claude', 'codex', 'gemini']);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npm test -- src/search/filter.test.ts`
Expected: FAIL — cannot resolve `./filter`.

- [ ] **Step 3: Implement**

`web/src/search/filter.ts`:
```ts
import type {
  LeanArtifact,
  ArtifactType,
  Maturity,
  Runtime,
} from '../types/registry';

export interface Facets {
  readonly query: string;
  readonly type: ArtifactType | undefined;
  readonly tag: string | undefined;
  readonly category: string | undefined;
  readonly runtime: Runtime | undefined;
  readonly maturity: Maturity | undefined;
}

export interface FacetValues {
  readonly types: readonly ArtifactType[];
  readonly tags: readonly string[];
  readonly categories: readonly string[];
  readonly runtimes: readonly Runtime[];
  readonly maturities: readonly Maturity[];
}

const RUNTIME_ORDER: readonly Runtime[] = ['claude', 'codex', 'gemini'];

const supportedOn = (a: LeanArtifact, rt: Runtime): boolean => {
  const level = a.support[rt];
  return level === 'native' || level === 'emulated';
};

export function filterArtifacts(
  artifacts: readonly LeanArtifact[],
  f: Facets,
): readonly LeanArtifact[] {
  const q = f.query.trim().toLowerCase();
  return artifacts.filter((a) => {
    // Deprecated excluded from default search (spec §11) unless explicitly chosen.
    if (a.maturity === 'deprecated' && f.maturity !== 'deprecated') return false;
    if (f.type && a.type !== f.type) return false;
    if (f.category && a.category !== f.category) return false;
    if (f.maturity && a.maturity !== f.maturity) return false;
    if (f.tag && !a.tags.includes(f.tag)) return false;
    if (f.runtime && !supportedOn(a, f.runtime)) return false;
    if (q && !`${a.name} ${a.description}`.toLowerCase().includes(q)) return false;
    return true;
  });
}

const uniqueSorted = <T extends string>(xs: readonly T[]): readonly T[] =>
  [...new Set(xs)].sort();

export function collectFacets(artifacts: readonly LeanArtifact[]): FacetValues {
  const runtimes = RUNTIME_ORDER.filter((rt) =>
    artifacts.some((a) => a.support[rt] !== undefined),
  );
  return {
    types: uniqueSorted(artifacts.map((a) => a.type)),
    tags: uniqueSorted(artifacts.flatMap((a) => a.tags)),
    categories: uniqueSorted(artifacts.map((a) => a.category)),
    runtimes,
    maturities: uniqueSorted(artifacts.map((a) => a.maturity)),
  };
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd web && npm test -- src/search/filter.test.ts`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add web/src/search/
git commit -m "feat(web): pure faceted search filter + facet collector"
```

---

### Task 7: Per-surface install instructions (pure generator)

**Files:**
- Create: `web/src/install/snippets.ts`
- Test: `web/src/install/snippets.test.ts`

> Surfaces (spec §8/§9/§10): Claude Code installs natively via `/plugin marketplace add` + `/plugin install <bundle>`; Codex/Gemini install via the `stark` CLI. Per-surface snippets vary by runtime support.

- [ ] **Step 1: Write failing tests**

`web/src/install/snippets.test.ts`:
```ts
import { describe, it, expect } from 'vitest';
import { installSnippets } from './snippets';

describe('installSnippets', () => {
  it('claude native uses /plugin marketplace add + /plugin install', () => {
    const s = installSnippets({ bundle: 'stark-review', runtime: 'claude', support: 'native' });
    expect(s.surface).toBe('claude-code');
    expect(s.commands.join('\n')).toContain('/plugin marketplace add 21-Stark-AI/stark-marketplace');
    expect(s.commands.join('\n')).toContain('/plugin install stark-review');
  });

  it('codex uses the stark CLI with --runtime codex', () => {
    const s = installSnippets({ bundle: 'stark-review', runtime: 'codex', support: 'native' });
    expect(s.commands.some((c) => c.includes('stark install stark-review --runtime codex'))).toBe(true);
  });

  it('gemini emulated carries a verify note', () => {
    const s = installSnippets({ bundle: 'stark-review', runtime: 'gemini', support: 'emulated' });
    expect(s.commands.some((c) => c.includes('--runtime gemini'))).toBe(true);
    expect(s.note).toMatch(/emulated/i);
  });

  it('per-artifact install scopes to bundle/artifact', () => {
    const s = installSnippets({ bundle: 'stark-review', artifact: 'review', runtime: 'codex', support: 'native' });
    expect(s.commands.some((c) => c.includes('stark install stark-review/review --runtime codex'))).toBe(true);
  });

  it('unsupported yields no commands and an explanatory note', () => {
    const s = installSnippets({ bundle: 'b', runtime: 'gemini', support: 'unsupported' });
    expect(s.commands).toEqual([]);
    expect(s.note).toMatch(/not supported/i);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npm test -- src/install/snippets.test.ts`
Expected: FAIL — cannot resolve `./snippets`.

- [ ] **Step 3: Implement**

`web/src/install/snippets.ts`:
```ts
import type { Runtime, SupportLevel } from '../types/registry';

export interface InstallTarget {
  readonly bundle: string;
  readonly artifact?: string;
  readonly runtime: Runtime;
  readonly support: SupportLevel;
}

export interface InstallSnippet {
  readonly surface: 'claude-code' | 'stark-cli';
  readonly commands: readonly string[];
  readonly note?: string;
}

const MARKETPLACE = '21-Stark-AI/stark-marketplace';

const target = (t: InstallTarget): string =>
  t.artifact ? `${t.bundle}/${t.artifact}` : t.bundle;

export function installSnippets(t: InstallTarget): InstallSnippet {
  if (t.support === 'unsupported') {
    return { surface: 'stark-cli', commands: [], note: `${t.type ?? 'This artifact'} is not supported on ${t.runtime}.` };
  }
  if (t.runtime === 'claude') {
    // Claude Code installs natively from the committed dist tree (spec §8).
    const cmds = [`/plugin marketplace add ${MARKETPLACE}`, `/plugin install ${t.bundle}`];
    return { surface: 'claude-code', commands: cmds };
  }
  // Codex + Gemini install via the stark CLI (spec §9).
  const cmd = `stark install ${target(t)} --runtime ${t.runtime}`;
  const note =
    t.support === 'emulated'
      ? `Emulated on ${t.runtime}: derived shape; may not auto-activate — verify after install.`
      : undefined;
  return note === undefined
    ? { surface: 'stark-cli', commands: [cmd] }
    : { surface: 'stark-cli', commands: [cmd], note };
}
```

> Note: the `t.type` reference in the unsupported branch requires an optional `type` on `InstallTarget`. Add `readonly type?: string;` to the interface so the message reads naturally; the test passes either way since it only matches `/not supported/i`.

Add to the interface:
```ts
  readonly type?: string;
```

- [ ] **Step 4: Run to verify pass**

Run: `cd web && npm test -- src/install/snippets.test.ts && npm run typecheck`
Expected: PASS (5 tests); typecheck clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/install/
git commit -m "feat(web): per-surface install snippet generator (CC native + stark CLI)"
```

---

### Task 8: Search page + facet controls + router bootstrap

**Files:**
- Create: `web/src/components/Facets.tsx`
- Create: `web/src/pages/SearchPage.tsx`
- Create: `web/src/pages/DegradedPage.tsx`
- Create: `web/src/app.tsx`
- Modify: `web/src/main.tsx`
- Test: `web/src/pages/SearchPage.test.tsx`

- [ ] **Step 1: Write the failing component test**

`web/src/pages/SearchPage.test.tsx`:
```tsx
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import indexFixture from '../__fixtures__/index.json';
import { isLeanIndex } from '../types/registry';
import { SearchPage } from './SearchPage';

const index = (() => {
  if (!isLeanIndex(indexFixture)) throw new Error('bad fixture');
  return indexFixture;
})();

const renderPage = () =>
  render(
    <MemoryRouter>
      <SearchPage index={index} />
    </MemoryRouter>,
  );

describe('SearchPage', () => {
  it('renders every artifact initially', () => {
    renderPage();
    expect(screen.getByText('stark-review')).toBeInTheDocument();
    expect(screen.getByText('gh')).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')).toHaveLength(3);
  });

  it('filters by the text query', () => {
    renderPage();
    fireEvent.change(screen.getByPlaceholderText(/search/i), { target: { value: 'github' } });
    expect(screen.getAllByRole('listitem')).toHaveLength(1);
    expect(screen.getByText('gh')).toBeInTheDocument();
  });

  it('filters by the type facet', () => {
    renderPage();
    fireEvent.change(screen.getByLabelText(/type/i), { target: { value: 'mcp' } });
    expect(screen.getAllByRole('listitem')).toHaveLength(1);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npm test -- src/pages/SearchPage.test.tsx`
Expected: FAIL — cannot resolve `./SearchPage`.

- [ ] **Step 3: Implement Facets + SearchPage + DegradedPage + app + main**

`web/src/components/Facets.tsx`:
```tsx
import type { Facets, FacetValues } from '../search/filter';

interface Props {
  readonly values: FacetValues;
  readonly facets: Facets;
  readonly onChange: (next: Facets) => void;
}

export function FacetControls({ values, facets, onChange }: Props): JSX.Element {
  return (
    <div className="facets">
      <input
        type="search"
        placeholder="Search artifacts…"
        value={facets.query}
        onChange={(e) => onChange({ ...facets, query: e.target.value })}
      />
      <label>
        Type
        <select
          value={facets.type ?? ''}
          onChange={(e) => onChange({ ...facets, type: (e.target.value || undefined) as Facets['type'] })}
        >
          <option value="">all</option>
          {values.types.map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
      </label>
      <label>
        Runtime
        <select
          value={facets.runtime ?? ''}
          onChange={(e) => onChange({ ...facets, runtime: (e.target.value || undefined) as Facets['runtime'] })}
        >
          <option value="">all</option>
          {values.runtimes.map((r) => (
            <option key={r} value={r}>{r}</option>
          ))}
        </select>
      </label>
      <label>
        Maturity
        <select
          value={facets.maturity ?? ''}
          onChange={(e) => onChange({ ...facets, maturity: (e.target.value || undefined) as Facets['maturity'] })}
        >
          <option value="">all</option>
          {values.maturities.map((m) => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>
      </label>
    </div>
  );
}
```

`web/src/pages/SearchPage.tsx`:
```tsx
import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import type { LeanIndex } from '../types/registry';
import { filterArtifacts, collectFacets, type Facets } from '../search/filter';
import { FacetControls } from '../components/Facets';

const EMPTY: Facets = {
  query: '', type: undefined, tag: undefined, category: undefined, runtime: undefined, maturity: undefined,
};

export function SearchPage({ index }: { readonly index: LeanIndex }): JSX.Element {
  const [facets, setFacets] = useState<Facets>(EMPTY);
  const facetValues = useMemo(() => collectFacets(index.artifacts), [index]);
  const results = useMemo(() => filterArtifacts(index.artifacts, facets), [index, facets]);

  return (
    <main>
      <h1>stark-marketplace</h1>
      <FacetControls values={facetValues} facets={facets} onChange={setFacets} />
      <ul>
        {results.map((a) => (
          <li key={`${a.bundle}/${a.type}/${a.name}`}>
            <Link to={`/bundle/${a.bundle}`}>{a.name}</Link>
            <span className="type">{a.type}</span>
            <span className="desc">{a.description}</span>
          </li>
        ))}
      </ul>
    </main>
  );
}
```

`web/src/pages/DegradedPage.tsx`:
```tsx
import type { DegradeReason } from '../data/registry';
import { registryError } from '../data/registry';

export function DegradedPage({ reason, githubUrl }: { readonly reason: DegradeReason; readonly githubUrl: string }): JSX.Element {
  return (
    <main role="alert">
      <h1>Registry unavailable</h1>
      <p>{registryError(reason)}</p>
      <a href={githubUrl}>Open the source on GitHub</a>
    </main>
  );
}
```

`web/src/app.tsx`:
```tsx
import { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { loadIndex, type IndexResult } from './data/registry';
import { SearchPage } from './pages/SearchPage';
import { DegradedPage } from './pages/DegradedPage';
import { BundleDetailPage } from './pages/BundleDetailPage';

export function App(): JSX.Element {
  const [state, setState] = useState<IndexResult | 'loading'>('loading');
  useEffect(() => {
    let active = true;
    void loadIndex().then((r) => { if (active) setState(r); });
    return () => { active = false; };
  }, []);

  if (state === 'loading') return <main aria-busy="true">Loading registry…</main>;
  if (state.kind === 'degraded') return <DegradedPage reason={state.reason} githubUrl={state.githubUrl} />;

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<SearchPage index={state.index} />} />
        <Route path="/bundle/:name" element={<BundleDetailPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
```

Update `web/src/main.tsx`:
```tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './app';

const el = document.getElementById('root');
if (el) {
  createRoot(el).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}
```

> `BundleDetailPage` is created in Task 9; to keep this task compiling on its own, add a minimal placeholder now and replace it in Task 9:
> `web/src/pages/BundleDetailPage.tsx`:
> ```tsx
> export function BundleDetailPage(): JSX.Element { return <main>detail</main>; }
> ```

- [ ] **Step 4: Run to verify pass**

Run: `cd web && npm test -- src/pages/SearchPage.test.tsx && npm run typecheck`
Expected: PASS (3 tests); typecheck clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/Facets.tsx web/src/pages/SearchPage.tsx web/src/pages/DegradedPage.tsx web/src/app.tsx web/src/main.tsx web/src/pages/BundleDetailPage.tsx
git commit -m "feat(web): faceted search page + router bootstrap + degraded view"
```

---

### Task 9: Support badges + install instructions + bundle detail page

**Files:**
- Create: `web/src/components/SupportBadges.tsx`
- Create: `web/src/components/InstallInstructions.tsx`
- Replace: `web/src/pages/BundleDetailPage.tsx`
- Test: `web/src/components/SupportBadges.test.tsx`
- Test: `web/src/pages/BundleDetailPage.test.tsx`

- [ ] **Step 1: Write failing tests**

`web/src/components/SupportBadges.test.tsx`:
```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SupportBadges } from './SupportBadges';

describe('SupportBadges', () => {
  it('renders a badge per runtime with the support level', () => {
    render(<SupportBadges support={{ claude: 'native', codex: 'native', gemini: 'emulated' }} />);
    expect(screen.getByText(/claude/i)).toHaveAttribute('data-support', 'native');
    expect(screen.getByText(/gemini/i)).toHaveAttribute('data-support', 'emulated');
  });

  // Defensive: the engine may emit a partial support map (e.g. before plan 03 fully
  // populates codex/gemini). Rendering a partial map must not crash and must skip
  // absent runtimes — guards the `support[rt] !== undefined` filter in the component.
  it('renders a partially-populated support map without crashing', () => {
    render(<SupportBadges support={{ claude: 'native' }} />);
    expect(screen.getByText(/claude/i)).toHaveAttribute('data-support', 'native');
    expect(screen.queryByText(/codex/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/gemini/i)).not.toBeInTheDocument();
  });
});
```

`web/src/pages/BundleDetailPage.test.tsx`:
```tsx
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import detailFixture from '../__fixtures__/bundles/stark-review.json';
import { BundleDetailPage } from './BundleDetailPage';

afterEach(() => vi.restoreAllMocks());

const renderAt = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/bundle/:name" element={<BundleDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );

describe('BundleDetailPage', () => {
  it('fetches and renders bundle + artifacts + install + source link', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200, json: async () => detailFixture,
    } as Response));

    renderAt('/bundle/stark-review');

    await waitFor(() => expect(screen.getByRole('heading', { name: 'stark-review' })).toBeInTheDocument());
    expect(screen.getByText(/Multi-agent PR review toolkit/)).toBeInTheDocument();
    // per-surface install for Claude
    expect(screen.getByText(/\/plugin install stark-review/)).toBeInTheDocument();
    // derived display output path (CC-3: outputs[rt][0].path), not a flat outputPaths map
    expect(screen.getByText('skills/stark-review/SKILL.md')).toBeInTheDocument();
    // deep link to GitHub source for an artifact
    const link = screen.getByRole('link', { name: /catalog\/stark-review\/skills\/stark-review.md/ });
    expect(link).toHaveAttribute('href', expect.stringContaining('github.com/21-Stark-AI/stark-marketplace'));
  });

  it('shows the degraded view on a fetch failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503, json: async () => null } as Response));
    renderAt('/bundle/stark-review');
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npm test -- src/components/SupportBadges.test.tsx src/pages/BundleDetailPage.test.tsx`
Expected: FAIL — undefined `SupportBadges`; placeholder detail page lacks the content.

- [ ] **Step 3: Implement**

`web/src/components/SupportBadges.tsx`:
```tsx
import type { SupportMatrix, Runtime } from '../types/registry';

const ORDER: readonly Runtime[] = ['claude', 'codex', 'gemini'];

export function SupportBadges({ support }: { readonly support: SupportMatrix }): JSX.Element {
  return (
    <span className="support-badges">
      {ORDER.filter((rt) => support[rt] !== undefined).map((rt) => (
        <span key={rt} className="badge" data-support={support[rt]}>
          {rt}: {support[rt]}
        </span>
      ))}
    </span>
  );
}
```

`web/src/components/InstallInstructions.tsx`:
```tsx
import type { Runtime, SupportLevel } from '../types/registry';
import { installSnippets } from '../install/snippets';

interface Props {
  readonly bundle: string;
  readonly artifact?: string;
  readonly type?: string;
  readonly support: Partial<Record<Runtime, SupportLevel>>;
}

const RUNTIMES: readonly Runtime[] = ['claude', 'codex', 'gemini'];

export function InstallInstructions({ bundle, artifact, type, support }: Props): JSX.Element {
  return (
    <div className="install">
      {RUNTIMES.filter((rt) => support[rt] !== undefined).map((rt) => {
        const level = support[rt] as SupportLevel;
        const snip = installSnippets({ bundle, artifact, type, runtime: rt, support: level });
        return (
          <section key={rt}>
            <h4>{rt} ({snip.surface})</h4>
            {snip.commands.length > 0 ? (
              <pre><code>{snip.commands.join('\n')}</code></pre>
            ) : null}
            {snip.note ? <p className="note">{snip.note}</p> : null}
          </section>
        );
      })}
    </div>
  );
}
```

`web/src/pages/BundleDetailPage.tsx` (replace the placeholder):
```tsx
import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { loadBundleDetail, type DetailResult } from '../data/registry';
import { outputPathFor, type DetailArtifact, type Runtime } from '../types/registry';
import { SupportBadges } from '../components/SupportBadges';
import { InstallInstructions } from '../components/InstallInstructions';
import { DependencyGraph } from '../components/DependencyGraph';
import { DegradedPage } from './DegradedPage';

const RUNTIME_ORDER: readonly Runtime[] = ['claude', 'codex', 'gemini'];

// Display path per runtime = first engine-emitted output path (CC-3 derivation).
function OutputPaths({ artifact }: { readonly artifact: DetailArtifact }): JSX.Element {
  const rows = RUNTIME_ORDER
    .map((rt) => [rt, outputPathFor(artifact, rt)] as const)
    .filter((r): r is readonly [Runtime, string] => r[1] !== undefined);
  if (rows.length === 0) return <p className="outputs empty">No emitted outputs.</p>;
  return (
    <ul className="outputs">
      {rows.map(([rt, path]) => (
        <li key={rt}><span className="rt">{rt}</span>: <code>{path}</code></li>
      ))}
    </ul>
  );
}

const SOURCE_TREE = 'https://github.com/21-Stark-AI/stark-marketplace/tree/main/';

export function BundleDetailPage(): JSX.Element {
  const { name } = useParams<{ name: string }>();
  const [state, setState] = useState<DetailResult | 'loading'>('loading');

  useEffect(() => {
    if (!name) return;
    let active = true;
    void loadBundleDetail(name).then((r) => { if (active) setState(r); });
    return () => { active = false; };
  }, [name]);

  if (state === 'loading') return <main aria-busy="true">Loading bundle…</main>;
  if (state.kind === 'degraded') return <DegradedPage reason={state.reason} githubUrl={state.githubUrl} />;

  const { bundle, artifacts, dependencyClosure } = state.detail;
  return (
    <main>
      <p><Link to="/">← back to search</Link></p>
      <h1>{bundle.name}</h1>
      <p>{bundle.description}</p>
      <p>v{bundle.version} · {bundle.maturity} · {bundle.category}</p>
      {bundle.homepage ? <a href={bundle.homepage}>source on GitHub</a> : null}

      <h2>Install</h2>
      <InstallInstructions
        bundle={bundle.name}
        support={artifacts.reduce<Record<string, never>>(() => ({}), {}) as never}
      />

      <h2>Artifacts</h2>
      {artifacts.map((a) => (
        <article key={a.name}>
          <h3>{a.name} <span className="type">{a.type}</span></h3>
          <p>{a.description}</p>
          <SupportBadges support={a.support} />
          <h4>Output paths</h4>
          <OutputPaths artifact={a} />
          <InstallInstructions bundle={bundle.name} artifact={a.name} type={a.type} support={a.support} />
          <a href={`${SOURCE_TREE}${a.sourcePath}`}>{a.sourcePath}</a>
        </article>
      ))}

      <h2>Dependencies</h2>
      <DependencyGraph edges={dependencyClosure} />
    </main>
  );
}
```

> The bundle-level `<InstallInstructions>` above passes an empty support map (bundle install is Claude-native + per-runtime CLI; the meaningful per-runtime detail is on each artifact). Simplify: render bundle-level Claude install only.

Replace the bundle-level install block with:
```tsx
      <h2>Install (whole bundle)</h2>
      <InstallInstructions bundle={bundle.name} support={{ claude: 'native', codex: 'native', gemini: 'native' }} />
```

> `DependencyGraph` is created in Task 10; add a placeholder now and replace it there:
> `web/src/components/DependencyGraph.tsx`:
> ```tsx
> import type { DependencyEdge } from '../types/registry';
> export function DependencyGraph({ edges }: { readonly edges: readonly DependencyEdge[] }): JSX.Element {
>   return <ul>{edges.map((e) => <li key={`${e.from}->${e.to}`}>{e.from} → {e.to}</li>)}</ul>;
> }
> ```

- [ ] **Step 4: Run to verify pass**

Run: `cd web && npm test -- src/components/SupportBadges.test.tsx src/pages/BundleDetailPage.test.tsx && npm run typecheck`
Expected: PASS (4 tests — 2 SupportBadges incl. the partial-map defensive case, 2 BundleDetailPage); typecheck clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SupportBadges.tsx web/src/components/InstallInstructions.tsx web/src/components/DependencyGraph.tsx web/src/pages/BundleDetailPage.tsx
git commit -m "feat(web): bundle detail page — support badges, per-surface install, source deep links"
```

---

### Task 10: Dependency graph (adjacency builder + rendering)

**Files:**
- Create: `web/src/graph/deps.ts`
- Replace: `web/src/components/DependencyGraph.tsx`
- Test: `web/src/graph/deps.test.ts`
- Test: `web/src/components/DependencyGraph.test.tsx`

- [ ] **Step 1: Write failing tests**

`web/src/graph/deps.test.ts`:
```ts
import { describe, it, expect } from 'vitest';
import { buildAdjacency, topoLayers } from './deps';
import type { DependencyEdge } from '../types/registry';

const edges: readonly DependencyEdge[] = [
  { from: 'a', to: 'b' },
  { from: 'a', to: 'c' },
  { from: 'b', to: 'd' },
];

describe('buildAdjacency', () => {
  it('maps each node to its direct dependencies (sorted)', () => {
    const adj = buildAdjacency(edges);
    expect(adj.get('a')).toEqual(['b', 'c']);
    expect(adj.get('b')).toEqual(['d']);
    expect(adj.has('d')).toBe(true);
  });
});

describe('topoLayers', () => {
  it('groups nodes into dependency layers (roots first)', () => {
    const layers = topoLayers(edges);
    expect(layers[0]).toContain('a');
    expect(layers[layers.length - 1]).toContain('d');
  });
  it('handles an empty graph', () => {
    expect(topoLayers([])).toEqual([]);
  });
});
```

`web/src/components/DependencyGraph.test.tsx`:
```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DependencyGraph } from './DependencyGraph';

describe('DependencyGraph', () => {
  it('renders each edge', () => {
    render(<DependencyGraph edges={[{ from: 'a', to: 'b' }]} />);
    expect(screen.getByText(/a → b/)).toBeInTheDocument();
  });
  it('renders an empty-state message with no edges', () => {
    render(<DependencyGraph edges={[]} />);
    expect(screen.getByText(/no dependencies/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npm test -- src/graph/deps.test.ts src/components/DependencyGraph.test.tsx`
Expected: FAIL — cannot resolve `./deps`; empty-state not handled.

- [ ] **Step 3: Implement**

`web/src/graph/deps.ts`:
```ts
import type { DependencyEdge } from '../types/registry';

/** Maps node → sorted direct dependencies. Every referenced node is a key. */
export function buildAdjacency(edges: readonly DependencyEdge[]): Map<string, string[]> {
  const adj = new Map<string, string[]>();
  const ensure = (n: string): string[] => {
    const existing = adj.get(n);
    if (existing) return existing;
    const fresh: string[] = [];
    adj.set(n, fresh);
    return fresh;
  };
  for (const e of edges) {
    ensure(e.from).push(e.to);
    ensure(e.to);
  }
  for (const deps of adj.values()) deps.sort();
  return adj;
}

/** Kahn-style layering; roots (no incoming) first. Cycle-safe: leftover nodes
 *  are appended as a final layer so rendering never hangs. */
export function topoLayers(edges: readonly DependencyEdge[]): string[][] {
  const adj = buildAdjacency(edges);
  const indegree = new Map<string, number>();
  for (const n of adj.keys()) indegree.set(n, 0);
  for (const deps of adj.values()) for (const d of deps) indegree.set(d, (indegree.get(d) ?? 0) + 1);

  const layers: string[][] = [];
  const remaining = new Set(adj.keys());
  while (remaining.size > 0) {
    const layer = [...remaining].filter((n) => (indegree.get(n) ?? 0) === 0).sort();
    if (layer.length === 0) {
      layers.push([...remaining].sort()); // cycle fallback
      break;
    }
    for (const n of layer) {
      remaining.delete(n);
      for (const d of adj.get(n) ?? []) indegree.set(d, (indegree.get(d) ?? 1) - 1);
    }
    layers.push(layer);
  }
  return layers;
}
```

`web/src/components/DependencyGraph.tsx` (replace the placeholder):
```tsx
import type { DependencyEdge } from '../types/registry';
import { topoLayers } from '../graph/deps';

export function DependencyGraph({ edges }: { readonly edges: readonly DependencyEdge[] }): JSX.Element {
  if (edges.length === 0) return <p className="empty">No dependencies.</p>;
  const layers = topoLayers(edges);
  return (
    <div className="dep-graph">
      <ol className="dep-layers">
        {layers.map((layer, i) => (
          <li key={i} className="dep-layer">{layer.join(', ')}</li>
        ))}
      </ol>
      <ul className="dep-edges">
        {edges.map((e) => (
          <li key={`${e.from}->${e.to}`}>{e.from} → {e.to}</li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd web && npm test -- src/graph/deps.test.ts src/components/DependencyGraph.test.tsx && npm run typecheck`
Expected: PASS (4 tests); typecheck clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/graph/ web/src/components/DependencyGraph.tsx
git commit -m "feat(web): dependency graph adjacency + layered rendering (cycle-safe)"
```

---

### Task 11: End-to-end smoke test (build against fixtures) + graceful-degrade smoke

**Files:**
- Test: `web/src/app.smoke.test.tsx`

- [ ] **Step 1: Write the failing smoke test (search render + detail render + degrade)**

`web/src/app.smoke.test.tsx`:
```tsx
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import indexFixture from './__fixtures__/index.json';
import detailFixture from './__fixtures__/bundles/stark-review.json';
import skewed from './__fixtures__/index.skewed.json';
import { App } from './app';

afterEach(() => vi.restoreAllMocks());

const routedFetch = (index: unknown, detail: unknown) =>
  vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    const body = url.includes('/bundles/') ? detail : index;
    return { ok: true, status: 200, json: async () => body } as Response;
  });

describe('SPA smoke', () => {
  it('boots against the fixture index, searches, and opens a detail page', async () => {
    vi.stubGlobal('fetch', routedFetch(indexFixture, detailFixture));

    render(<App />);

    // search renders
    await waitFor(() => expect(screen.getByText('stark-review')).toBeInTheDocument());
    expect(screen.getAllByRole('listitem')).toHaveLength(3);

    // navigate to detail
    fireEvent.click(screen.getAllByText('stark-review')[0]);
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'stark-review' })).toBeInTheDocument(),
    );
    expect(screen.getByText(/\/plugin install stark-review/)).toBeInTheDocument();
  });

  it('degrades gracefully (never blank) on a bumped schemaVersion index', async () => {
    vi.stubGlobal('fetch', routedFetch(skewed, detailFixture));
    render(<App />);
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.getByText(/github/i)).toBeInTheDocument();
    // assert we did NOT render the search heading
    expect(screen.queryByPlaceholderText(/search/i)).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails (or passes if all wiring is correct)**

Run: `cd web && npm test -- src/app.smoke.test.tsx`
Expected: FAIL first if any wiring gap exists (e.g. the detail Link/route); fix wiring until green. The two assertions exercise spec §10's "search + detail render" and "graceful degrade on schema skew."

- [ ] **Step 3: Make it pass**

If the detail navigation fails, ensure `SearchPage` links use `to={/bundle/${a.bundle}}` and `app.tsx` mounts `/bundle/:name` (Task 8). No new impl should be needed beyond Tasks 1–10; resolve any gap surfaced by the test.

- [ ] **Step 4: Run the full suite + typecheck + build**

Run:
```bash
cd web && npm test && npm run typecheck && npm run build && cd ..
```
Expected: ALL vitest files PASS; typecheck clean; `vite build` emits hashed `web/dist/assets/*` + `web/dist/index.html`.

- [ ] **Step 5: Commit**

```bash
git add web/src/app.smoke.test.tsx
git commit -m "test(web): SPA smoke — fixture boot, search+detail render, graceful degrade"
```

---

### Task 12: CI deploy workflow + gated-static hosting note + README

**Files:**
- Create: `.github/workflows/web-deploy.yml`
- Create: `docs/web-hosting.md`
- Create: `web/README.md`

> Deploy publishes **SPA + index as ONE atomic content-hashed unit**: hashed long-cache assets, with `index.html` (the cache-busted pointer) and `index.json`/`bundles/` copied in from the committed engine output, all uploaded together so a client never mixes an old SPA with a new index. Provisioning is documented, **not** executed here (spec §10, §15.5).

- [ ] **Step 1: Write the CI workflow**

`.github/workflows/web-deploy.yml`:
```yaml
name: web-deploy

on:
  push:
    branches: [main]
    paths: ['web/**', 'index.json', 'bundles/**', '.github/workflows/web-deploy.yml']
  workflow_dispatch: {}

permissions:
  contents: read
  id-token: write   # for keyless auth to the gated-static origin (OIDC)

jobs:
  build:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npm run typecheck
      - run: npm test
      - run: npm run build   # tsc --noEmit && vite build → web/dist with hashed assets
      # Stage the committed engine output INTO the same dist as the SPA so the
      # whole thing ships as one content-hashed unit (spec §10).
      - name: Stage index + per-bundle detail
        working-directory: ${{ github.workspace }}
        run: |
          set -euo pipefail
          mkdir -p web/dist/bundles
          cp index.json web/dist/index.json
          cp bundles/*.json web/dist/bundles/ 2>/dev/null || true
      - name: Upload SPA+index artifact (atomic unit)
        uses: actions/upload-artifact@v4
        with:
          name: web-dist
          path: web/dist
          if-no-files-found: error
      # Publish step is environment-specific and runs against the 21 Stark AI-standard
      # gated-static origin behind the identity-aware proxy. It is intentionally a
      # single atomic sync of web/dist (assets + index.html + index.json + bundles/)
      # — long-cache the hashed assets, no-cache index.html + index.json.
      #   Example (DO NOT enable until the origin is provisioned per docs/web-hosting.md):
      #   - run: |
      #       gsutil -m -h "Cache-Control:public,max-age=31536000,immutable" \
      #         rsync -r web/dist/assets gs://$BUCKET/assets
      #       gsutil -h "Cache-Control:no-cache" cp web/dist/index.html gs://$BUCKET/index.html
      #       gsutil -h "Cache-Control:no-cache" cp web/dist/index.json gs://$BUCKET/index.json
      #       gsutil -m -h "Cache-Control:no-cache" rsync -r web/dist/bundles gs://$BUCKET/bundles
```

- [ ] **Step 2: Write the hosting infra note**

`docs/web-hosting.md`:
```markdown
# stark-marketplace — Web registry hosting (gated-static behind SSO)

> **No ad-hoc provisioning.** This documents the target pattern. Provisioning lands
> through the standard 21 Stark AI IaC path (Terraform), not from this repo's CI.

## Pattern: identity-aware proxy in front of static content

- **Origin:** the atomic content-hashed `web/dist` bundle (SPA shell + hashed assets +
  `index.json` + `bundles/*.json`), produced by `.github/workflows/web-deploy.yml`.
- **Gate:** an **identity-aware proxy enforcing Google Workspace SSO** sits in front of
  the origin. Options that satisfy the spec:
  - **GCP Cloud Run + IAP** (Cloud Run serves the static bundle via a tiny static
    file server image; IAP enforces the 21 Stark AI Workspace org). Recommended.
  - Equivalent: GCLB + IAP in front of a GCS backend bucket.
- **Critical invariant (spec §10):** the proxy gates **ALL data files**, not just HTML —
  `index.json`, every `bundles/<name>.json`, and any served Claude tree are behind the
  same SSO check. There is **no anonymous origin** and **no app-level user store**;
  identity is the proxy's job.

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
```

- [ ] **Step 3: Write the web README**

`web/README.md`:
```markdown
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
gated on the 21 Stark AI-standard hosting origin (`../docs/web-hosting.md`).
```

- [ ] **Step 4: Validate the workflow YAML + docs build cleanly**

Run:
```bash
cd web && npm run build && cd .. && python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/web-deploy.yml')); print('workflow yaml ok')"
```
Expected: build succeeds; "workflow yaml ok" prints. (The `python3` line is a one-off YAML lint, not project code — no Python is added to the repo.)

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/web-deploy.yml docs/web-hosting.md web/README.md
git commit -m "ci(web): atomic SPA+index deploy workflow + gated-static SSO hosting note"
```

---

## Self-Review

**Spec coverage (§10 + cross-refs):**
- TypeScript + Vite static SPA under `web/`, strict + ESM + narrow types (no `any`, lint-enforced) — Tasks 1, 3. ✅
- Reads lean `index.json` for search + per-bundle `bundles/<name>.json` on demand; no app server for data — Tasks 5, 6, 9. ✅
- Faceted search (type/tag/category/runtime/maturity) — Task 6 (`filterArtifacts`/`collectFacets`), Task 8 (UI). ✅
- Bundle + artifact detail pages — Task 9. ✅
- Per-surface install instructions (CC `/plugin install`; `stark install` for codex/gemini) — Task 7, rendered Task 9. ✅
- Native/emulated support badges — Task 9 (`SupportBadges`). ✅
- Dependency graph — Task 10. ✅
- Deep links to GitHub source — Task 9 (artifact `sourcePath` + bundle `homepage`). ✅
- `schemaVersion` graceful degrade (N-1 support; never blank-fails; points to GitHub) — Tasks 4, 5, 8, 11. ✅
- Consumers ignore unknown fields (forward compat) — Task 3 guards + Task 5 parser. ✅
- Atomic content-hashed deploy (hashed long-cache assets; index pointer cache-busted with SPA build hash) — Tasks 1 (vite hashing), 12 (workflow + caching note). ✅
- Gated-static SSO behind identity-aware proxy gating ALL data files; no app-level user store; documented not provisioned — Task 12 (`docs/web-hosting.md`). ✅
- Tests: vitest component/unit; smoke build against fixture index renders search + detail; graceful-degrade test on bumped schemaVersion fixture — Tasks 2–11 (esp. Task 11 smoke + Task 4/5 degrade). ✅

**Type consistency with engine output (CC-2/CC-3):** `src/types/registry.ts` `LeanArtifact`/`LeanIndex` mirror the lean index CC-2 exactly — top-level key **`artifacts`** (not `entries`), each row carrying `description` (used by the full-text search in `filter.ts`), `schemaVersion` an int. `BundleDetail`/`DetailArtifact` mirror the engine-emitted CC-3 detail: per-runtime `support`, `requires`, `diverged`, `fidelityNotes`, and the **`outputs`** map of `{path, kind, key, sentinel, emulated}` arrays. The SPA never expects a flat `outputPaths` from the engine — it **derives** display paths via `outputPathFor(a, rt) = a.outputs[rt][0]?.path` (rendered by the detail page's `OutputPaths`). Fixtures (Task 3) are engine-shaped (generated from `stark build` or kept byte-aligned + asserted by the type guards) and are the executable check that the shapes match; if plan 02/03 reshapes a JSON field, regenerate `registry.ts` + fixtures together. The `SupportBadges` partial-map test (Task 9) hardens the UI against a partially-populated `support` map (engine may emit claude-only before plan 03's CC-4 enrichment lands).

**Placeholder scan:** No `TODO`/`FIXME`/`any`/stub-left-behind in shipped code. Two intentional placeholders (`BundleDetailPage` in Task 8, `DependencyGraph` in Task 9) are explicitly **replaced** in Tasks 9 and 10 respectively — they exist only to keep each task independently compilable, and the later task's `git add` overwrites them. The publish step in the CI workflow is intentionally commented and gated on `docs/web-hosting.md` provisioning (spec §10/§15.5 say document, don't provision).

**Handoff:** After Task 12, `web/` is a strict-TS Vite SPA with green vitest (unit + component + smoke + degrade), a passing `vite build` producing a content-hashed atomic bundle, and a documented SSO gated-static deploy — ready for plan 07 (migration) to populate the real catalog and plan 08 to wire the hosting origin via IaC.
