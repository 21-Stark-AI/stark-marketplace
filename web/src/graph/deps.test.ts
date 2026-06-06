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
