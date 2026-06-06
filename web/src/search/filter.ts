import type { LeanArtifact, ArtifactType, Maturity, Runtime } from '../types/registry';

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
    // Engine `omitempty` fields may be absent — default defensively so a tag-less or
    // description-less artifact never crashes or mis-filters the search.
    const tags = a.tags ?? [];
    // Deprecated excluded from default search (spec §11) unless explicitly chosen.
    if (a.maturity === 'deprecated' && f.maturity !== 'deprecated') return false;
    if (f.type && a.type !== f.type) return false;
    if (f.category && a.category !== f.category) return false;
    if (f.maturity && a.maturity !== f.maturity) return false;
    if (f.tag && !tags.includes(f.tag)) return false;
    if (f.runtime && !supportedOn(a, f.runtime)) return false;
    if (q && !`${a.name} ${a.description ?? ''}`.toLowerCase().includes(q)) return false;
    return true;
  });
}

const uniqueSorted = <T extends string>(xs: readonly (T | undefined)[]): readonly T[] =>
  [...new Set(xs.filter((x): x is T => x !== undefined))].sort();

export function collectFacets(artifacts: readonly LeanArtifact[]): FacetValues {
  const runtimes = RUNTIME_ORDER.filter((rt) =>
    artifacts.some((a) => a.support[rt] !== undefined),
  );
  return {
    types: uniqueSorted(artifacts.map((a) => a.type)),
    tags: uniqueSorted(artifacts.flatMap((a) => a.tags ?? [])),
    categories: uniqueSorted(artifacts.map((a) => a.category)),
    runtimes,
    maturities: uniqueSorted(artifacts.map((a) => a.maturity)),
  };
}
