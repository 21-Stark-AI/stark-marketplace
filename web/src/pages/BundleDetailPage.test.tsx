import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import detailFixture from '../__fixtures__/bundles/stark-review.json';
import { BundleDetailPage } from './BundleDetailPage';

afterEach(() => vi.restoreAllMocks());

const renderAt = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/bundle/:name" element={<BundleDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );

describe('BundleDetailPage', () => {
  it('fetches and renders bundle + artifacts + install + source link', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200, json: async () => detailFixture,
    } as Response));

    renderAt('/bundle/stark-review');

    await waitFor(() => expect(screen.getByRole('heading', { name: 'stark-review' })).toBeInTheDocument());
    expect(screen.getByText(/Multi-agent PR review toolkit/)).toBeInTheDocument();
    // per-surface install for Claude (shown for the whole bundle + each claude-native artifact)
    expect(screen.getAllByText(/\/plugin install stark-review/).length).toBeGreaterThan(0);
    // derived display output path (CC-3: outputs[rt][0].path), not a flat outputPaths map
    expect(screen.getByText('skills/stark-review/SKILL.md')).toBeInTheDocument();
    // deep link to GitHub source (derived from bundle.homepage — the engine emits no per-artifact sourcePath)
    const link = screen.getByRole('link', { name: /stark-review source on GitHub/ });
    expect(link).toHaveAttribute('href', expect.stringContaining('github.com/21-Stark-AI/stark-marketplace'));
    // dependency edge derived from per-artifact requires (no engine `dependencyClosure`)
    expect(screen.getByText(/stark-review\/stark-review → stark-review\/stark-session/)).toBeInTheDocument();
  });

  it('shows the degraded view on a fetch failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503, json: async () => null } as Response));
    renderAt('/bundle/stark-review');
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument());
  });
});
