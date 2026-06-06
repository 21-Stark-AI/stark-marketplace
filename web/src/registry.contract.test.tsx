import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
// Import the ACTUAL files the engine emits + CI ships (repo root), NOT a hand-authored fixture.
// This pins the SPA's types/guards/rendering to real engine output so contract drift fails CI.
import realIndex from '../../index.json';
import realDetail from '../../bundles/stark-gh.json';
import { isLeanIndex, isBundleDetail } from './types/registry';
import { BundleDetailPage } from './pages/BundleDetailPage';

afterEach(() => vi.restoreAllMocks());

describe('engine-contract fidelity (real committed data)', () => {
  it('isLeanIndex accepts the real committed index.json', () => {
    expect(isLeanIndex(realIndex)).toBe(true);
  });

  it('isBundleDetail accepts the real committed bundles/stark-gh.json', () => {
    expect(isBundleDetail(realDetail)).toBe(true);
  });

  // Regression for the dependencyClosure/sourcePath/owner-object class of bugs: the real detail
  // has no dependencyClosure, no per-artifact sourcePath, and a STRING owner. The page must
  // render it without throwing (it crashed/blanked before the contract fix).
  it('renders the real bundle detail without crashing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200, json: async () => realDetail,
    } as Response));
    const { findByRole } = render(
      <MemoryRouter initialEntries={['/bundle/stark-gh']}>
        <Routes>
          <Route path="/bundle/:name" element={<BundleDetailPage />} />
        </Routes>
      </MemoryRouter>,
    );
    await findByRole('heading', { name: 'stark-gh', level: 1 });
    // The render reaches the derived dependency section (real stark-gh has requires:[] → empty
    // state) — proving the dependencyClosure→requires migration handles real engine data, where
    // the old code crashed reading a non-existent dependencyClosure field.
    expect(screen.getByText(/no dependencies/i)).toBeInTheDocument();
    // and a real per-artifact derived output path renders (CC-3 outputs[rt][0].path)
    expect(screen.getByText('.mcp.json')).toBeInTheDocument();
  });
});
