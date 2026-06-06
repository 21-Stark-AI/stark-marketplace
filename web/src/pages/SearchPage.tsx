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
