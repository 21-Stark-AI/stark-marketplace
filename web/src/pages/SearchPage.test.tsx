import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import indexFixture from '../__fixtures__/index.json';
import { isLeanIndex } from '../types/registry';
import { SearchPage } from './SearchPage';

const index = (() => {
  if (!isLeanIndex(indexFixture)) throw new Error('bad fixture');
  return indexFixture;
})();

const renderPage = () =>
  render(
    <MemoryRouter>
      <SearchPage index={index} />
    </MemoryRouter>,
  );

describe('SearchPage', () => {
  it('renders every artifact initially', () => {
    renderPage();
    expect(screen.getByText('stark-review')).toBeInTheDocument();
    expect(screen.getByText('gh')).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')).toHaveLength(3);
  });

  it('filters by the text query', () => {
    renderPage();
    fireEvent.change(screen.getByPlaceholderText(/search/i), { target: { value: 'github' } });
    expect(screen.getAllByRole('listitem')).toHaveLength(1);
    expect(screen.getByText('gh')).toBeInTheDocument();
  });

  it('filters by the type facet', () => {
    renderPage();
    fireEvent.change(screen.getByLabelText(/type/i), { target: { value: 'mcp' } });
    expect(screen.getAllByRole('listitem')).toHaveLength(1);
  });
});
