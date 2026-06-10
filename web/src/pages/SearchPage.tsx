import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import type { LeanIndex } from '../types/registry';
import { filterArtifacts, collectFacets, type Facets } from '../search/filter';
import { FacetControls } from '../components/Facets';
import { ProvenanceBadge } from '../components/ProvenanceBadge';
import { NavTabs } from '../components/NavTabs';

const EMPTY: Facets = {
  query: '', type: undefined, tag: undefined, category: undefined, runtime: undefined, maturity: undefined,
};

export function SearchPage({ index }: { readonly index: LeanIndex }): JSX.Element {
  const [facets, setFacets] = useState<Facets>(EMPTY);
  const facetValues = useMemo(() => collectFacets(index.artifacts), [index]);
  const results = useMemo(() => filterArtifacts(index.artifacts, facets), [index, facets]);

  return (
    <main>
      <header className="hero">
        <p className="hero-kicker">stark ecosystem · multi-runtime registry</p>
        <h1>
          stark<span className="accent">·</span>marketplace
          <span className="mono-tag">claude / codex / gemini</span>
        </h1>
        <p className="hero-sub">
          One source of truth, rendered into every runtime. Browse, filter, and install
          signed skill, command, agent, prompt &amp; MCP bundles.
        </p>
        <NavTabs />
        <ProvenanceBadge index={index} />
        <FacetControls values={facetValues} facets={facets} onChange={setFacets} />
      </header>
      <div className="content">
        <p className="results-meta">
          <b>{results.length}</b> {results.length === 1 ? 'artifact' : 'artifacts'}
          {results.length !== index.artifacts.length ? ` of ${index.artifacts.length}` : ''}
        </p>
        <ul>
          {results.map((a) => (
            <li key={`${a.bundle}/${a.type}/${a.name}`}>
              <Link to={`/bundle/${a.bundle}`}>{a.name}</Link>
              <span className="type">{a.type}</span>
              <span className="desc">{a.description}</span>
            </li>
          ))}
        </ul>
      </div>
    </main>
  );
}
