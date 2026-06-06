import { describe, it, expect } from 'vitest';
import { buildAdjacency, topoLayers, edgesFromArtifacts } from './deps';
import type { DependencyEdge, DetailArtifact } from '../types/registry';

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
  it('terminates on a cycle (never hangs) and surfaces the cyclic nodes', () => {
    const cyclic: readonly DependencyEdge[] = [
      { from: 'a', to: 'b' },
      { from: 'b', to: 'a' },
    ];
    const layers = topoLayers(cyclic); // must return, not loop forever
    expect(layers.flat().sort()).toEqual(['a', 'b']);
  });
});

describe('edgesFromArtifacts', () => {
  const artifacts: readonly DetailArtifact[] = [
    {
      name: 'review', type: 'skill', version: '1', support: {}, outputs: {},
      requires: [{ type: 'skill', ref: 'session' }, { type: 'mcp', ref: 'other/gh' }],
    },
    { name: 'session', type: 'skill', version: '1', support: {}, outputs: {}, requires: [] },
  ];
  it('derives edges from per-artifact requires, resolving bare refs to the owning bundle', () => {
    const edges = edgesFromArtifacts('rev', artifacts);
    expect(edges).toEqual([
      { from: 'rev/review', to: 'rev/session' },
      { from: 'rev/review', to: 'other/gh' },
    ]);
  });
  it('returns no edges when no artifact has requires', () => {
    expect(edgesFromArtifacts('rev', [{ name: 'x', type: 'command', version: '1', support: {}, outputs: {} }])).toEqual([]);
  });
});
