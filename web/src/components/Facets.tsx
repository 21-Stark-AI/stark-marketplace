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
        aria-label="Search artifacts"
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
