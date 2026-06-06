// Mirrors the engine's ACTUAL emitted JSON (spec §7.5, CC-2/CC-3) and the Go model
// (engine/internal/index/index.go). Consumers IGNORE unknown fields; conversely, fields the
// engine emits with `omitempty` are modeled OPTIONAL here so a missing key never crashes the
// SPA (forward/back compat). These shapes were reconciled against the committed index.json +
// bundles/stark-gh.json — the contract test (registry.contract.test.ts) pins them to real data.

export type Runtime = 'claude' | 'codex' | 'gemini';
export type ArtifactType = 'skill' | 'prompt' | 'command' | 'agent' | 'mcp';
export type Maturity = 'experimental' | 'beta' | 'stable' | 'deprecated';
export type SupportLevel = 'native' | 'emulated' | 'unsupported';

export type SupportMatrix = Partial<Record<Runtime, SupportLevel>>;

/** One row of the lean index.json — only search-facing fields (spec §7.5). Engine `omitempty`
 *  fields (tags/category/maturity/description) are optional: an artifact may legitimately omit
 *  them, so the search code must not assume their presence. */
export interface LeanArtifact {
  readonly name: string;
  readonly type: ArtifactType;
  readonly bundle: string;
  readonly description?: string;
  readonly tags?: readonly string[];
  readonly category?: string;
  readonly maturity?: Maturity;
  readonly version: string;
  readonly digest?: string;
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

/** Bundle-level metadata (CC-3 `bundle`). `owner` is a plain STRING in the engine output
 *  (b.Owner.Name), NOT an object; the bundle has no `runtimes` field (runtimes live per
 *  artifact). */
export interface BundleMeta {
  readonly name: string;
  readonly version: string;
  readonly description?: string;
  readonly category?: string;
  readonly tags?: readonly string[];
  readonly maturity?: Maturity;
  readonly owner?: string;
  readonly homepage?: string;
}

export type OutputKind = 'file' | 'mergeJSONKey' | 'mergeTOMLKey' | 'sentinel';

/** One engine-emitted output for a (artifact, runtime). Mirrors CC-3 `outputs[]`. `key` and
 *  `sentinel` are `omitempty` in the engine, so they are ABSENT (not null) for a plain file. */
export interface ArtifactOutput {
  readonly path: string;
  readonly kind: OutputKind;
  readonly key?: string; // merge target key for merge* kinds
  readonly sentinel?: string; // sentinel name for sentinel kind
  readonly emulated: boolean;
}

export type OutputMatrix = Partial<Record<Runtime, readonly ArtifactOutput[]>>;

/** Per-artifact detail (CC-3). The engine does NOT emit tags/maturity/sourcePath on the detail
 *  artifact — only on the lean index row — so they are absent here. */
export interface DetailArtifact {
  readonly name: string;
  readonly type: ArtifactType;
  readonly description?: string;
  readonly version: string;
  readonly runtimes?: readonly Runtime[];
  readonly requires?: readonly Requirement[];
  readonly support: SupportMatrix;
  readonly diverged?: boolean;
  // Engine-emitted per-runtime outputs (CC-3). Display paths are DERIVED, not read flat.
  readonly outputs: OutputMatrix;
  readonly fidelityNotes?: Partial<Record<Runtime, string>>;
}

/** Derive the display path for a runtime = first emitted output's path (CC-3). */
export function outputPathFor(a: DetailArtifact, rt: Runtime): string | undefined {
  return a.outputs[rt]?.[0]?.path;
}

export interface DependencyEdge {
  readonly from: string;
  readonly to: string;
}

/** Per-bundle bundles/<name>.json document. The engine emits exactly schemaVersion + bundle +
 *  artifacts — there is NO `dependencyClosure`; the SPA DERIVES the dep graph from per-artifact
 *  `requires` (see graph/deps.ts `edgesFromArtifacts`). */
export interface BundleDetail {
  readonly schemaVersion: number;
  readonly bundle: BundleMeta;
  readonly artifacts: readonly DetailArtifact[];
}

const isRecord = (v: unknown): v is Record<string, unknown> =>
  typeof v === 'object' && v !== null;

export function isLeanIndex(v: unknown): v is LeanIndex {
  return isRecord(v) && typeof v.schemaVersion === 'number' && Array.isArray(v.artifacts);
}

export function isBundleDetail(v: unknown): v is BundleDetail {
  return (
    isRecord(v) &&
    typeof v.schemaVersion === 'number' &&
    isRecord(v.bundle) &&
    Array.isArray(v.artifacts)
  );
}
