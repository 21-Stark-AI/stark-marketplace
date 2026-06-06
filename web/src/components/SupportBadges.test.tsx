import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SupportBadges } from './SupportBadges';

describe('SupportBadges', () => {
  it('renders a badge per runtime with the support level', () => {
    render(<SupportBadges support={{ claude: 'native', codex: 'native', gemini: 'emulated' }} />);
    expect(screen.getByText(/claude/i)).toHaveAttribute('data-support', 'native');
    expect(screen.getByText(/gemini/i)).toHaveAttribute('data-support', 'emulated');
  });

  // Defensive: the engine may emit a partial support map (e.g. before plan 03 fully
  // populates codex/gemini). Rendering a partial map must not crash and must skip
  // absent runtimes — guards the `support[rt] !== undefined` filter in the component.
  it('renders a partially-populated support map without crashing', () => {
    render(<SupportBadges support={{ claude: 'native' }} />);
    expect(screen.getByText(/claude/i)).toHaveAttribute('data-support', 'native');
    expect(screen.queryByText(/codex/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/gemini/i)).not.toBeInTheDocument();
  });
});
