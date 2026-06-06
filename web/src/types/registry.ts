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
  readonly key: string | null; // merge target key for merge* kinds
  readonly sentinel: string | null; // sentinel name for sentinel kind
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
