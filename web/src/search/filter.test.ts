import { describe, it, expect } from 'vitest';
import indexFixture from '../__fixtures__/index.json';
import { isLeanIndex } from '../types/registry';
import { filterArtifacts, collectFacets, type Facets } from './filter';

const index = (() => {
  if (!isLeanIndex(indexFixture)) throw new Error('bad fixture');
  return indexFixture;
})();

const empty: Facets = {
  query: '',
  type: undefined,
  tag: undefined,
  category: undefined,
  runtime: undefined,
  maturity: undefined,
};

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

// Forward-compat: the engine emits tags/category/maturity/description with omitempty, so an
// artifact may legitimately omit them. The pure search functions must not crash or inject
// undefined facet values.
describe('forward-compat with omitempty fields absent', () => {
  const sparse = [
    { name: 'bare', type: 'command' as const, bundle: 'b', version: '1', support: { claude: 'native' as const } },
  ];
  it('filterArtifacts does not crash on a tag-less artifact (tag facet active)', () => {
    expect(filterArtifacts(sparse, { ...empty, tag: 'anything' })).toEqual([]);
    expect(filterArtifacts(sparse, empty).length).toBe(1);
    expect(filterArtifacts(sparse, { ...empty, query: 'bare' }).length).toBe(1);
  });
  it('collectFacets omits undefined values from sparse rows', () => {
    const f = collectFacets(sparse);
    expect(f.tags).toEqual([]);
    expect(f.categories).toEqual([]);
    expect(f.maturities).toEqual([]);
    expect(f.types).toEqual(['command']);
  });
});
