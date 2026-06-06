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
