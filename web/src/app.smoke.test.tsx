import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import indexFixture from './__fixtures__/index.json';
import detailFixture from './__fixtures__/bundles/stark-review.json';
import skewed from './__fixtures__/index.skewed.json';
import { App } from './app';

afterEach(() => vi.restoreAllMocks());

const routedFetch = (index: unknown, detail: unknown) =>
  vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    const body = url.includes('/bundles/') ? detail : index;
    return { ok: true, status: 200, json: async () => body } as Response;
  });

describe('SPA smoke', () => {
  it('boots against the fixture index, searches, and opens a detail page', async () => {
    vi.stubGlobal('fetch', routedFetch(indexFixture, detailFixture));

    render(<App />);

    // search renders
    await waitFor(() => expect(screen.getByText('stark-review')).toBeInTheDocument());
    expect(screen.getAllByRole('listitem')).toHaveLength(3);

    // navigate to detail (getAllByText[0] is HTMLElement|undefined under noUncheckedIndexedAccess)
    const [firstMatch] = screen.getAllByText('stark-review');
    expect(firstMatch).toBeInTheDocument();
    fireEvent.click(firstMatch as HTMLElement);
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'stark-review' })).toBeInTheDocument(),
    );
    expect(screen.getAllByText(/\/plugin install stark-review/).length).toBeGreaterThan(0);
  });

  it('degrades gracefully (never blank) on a bumped schemaVersion index', async () => {
    vi.stubGlobal('fetch', routedFetch(skewed, detailFixture));
    render(<App />);
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument());
    // the degraded view points the user to the GitHub source (link, precise over text match)
    expect(screen.getByRole('link', { name: /github/i })).toBeInTheDocument();
    // assert we did NOT render the search heading
    expect(screen.queryByPlaceholderText(/search/i)).not.toBeInTheDocument();
  });
});
