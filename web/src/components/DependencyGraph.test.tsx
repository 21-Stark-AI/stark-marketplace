import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DependencyGraph } from './DependencyGraph';

describe('DependencyGraph', () => {
  it('renders each edge', () => {
    render(<DependencyGraph edges={[{ from: 'a', to: 'b' }]} />);
    expect(screen.getByText(/a → b/)).toBeInTheDocument();
  });
  it('renders an empty-state message with no edges', () => {
    render(<DependencyGraph edges={[]} />);
    expect(screen.getByText(/no dependencies/i)).toBeInTheDocument();
  });
});
