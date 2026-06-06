import type { DependencyEdge, DetailArtifact } from '../types/registry';

/** Derive dependency edges from per-artifact `requires` (the engine emits dependencies this
 *  way; there is no `dependencyClosure` in the detail JSON). A bare ref ("name") resolves to the
 *  owning bundle; a "bundle/name" ref is used as-is. */
export function edgesFromArtifacts(
  bundle: string,
  artifacts: readonly DetailArtifact[],
): DependencyEdge[] {
  const edges: DependencyEdge[] = [];
  for (const a of artifacts) {
    const from = `${bundle}/${a.name}`;
    for (const req of a.requires ?? []) {
      const to = req.ref.includes('/') ? req.ref : `${bundle}/${req.ref}`;
      edges.push({ from, to });
    }
  }
  return edges;
}

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
