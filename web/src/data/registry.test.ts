import { describe, it, expect, vi, afterEach } from 'vitest';
import indexFixture from '../__fixtures__/index.json';
import detailFixture from '../__fixtures__/bundles/stark-review.json';
import skewed from '../__fixtures__/index.skewed.json';
import { loadIndex, loadBundleDetail, registryError } from './registry';

const mockFetch = (status: number, body: unknown) =>
  vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response);

afterEach(() => vi.restoreAllMocks());

describe('loadIndex', () => {
  it('returns a parsed lean index on a supported version', async () => {
    vi.stubGlobal('fetch', mockFetch(200, indexFixture));
    const res = await loadIndex('/index.json');
    expect(res.kind).toBe('ok');
    if (res.kind === 'ok') expect(res.index.artifacts.length).toBe(3);
  });

  it('degrades (not throws) on a skewed schemaVersion', async () => {
    vi.stubGlobal('fetch', mockFetch(200, skewed));
    const res = await loadIndex('/index.json');
    expect(res.kind).toBe('degraded');
    if (res.kind === 'degraded') expect(res.reason).toBe('unsupported-newer');
  });

  it('degrades on a non-conforming payload', async () => {
    vi.stubGlobal('fetch', mockFetch(200, { nope: true }));
    const res = await loadIndex('/index.json');
    expect(res.kind).toBe('degraded');
  });

  it('degrades on an HTTP error (e.g. proxy 401/5xx)', async () => {
    vi.stubGlobal('fetch', mockFetch(503, null));
    const res = await loadIndex('/index.json');
    expect(res.kind).toBe('degraded');
    if (res.kind === 'degraded') expect(res.reason).toBe('fetch-failed');
  });
});

describe('loadBundleDetail', () => {
  it('returns parsed detail', async () => {
    vi.stubGlobal('fetch', mockFetch(200, detailFixture));
    const res = await loadBundleDetail('stark-review');
    expect(res.kind).toBe('ok');
    if (res.kind === 'ok') expect(res.detail.bundle.name).toBe('stark-review');
  });

  it('degrades (malformed) on a non-conforming detail payload', async () => {
    vi.stubGlobal('fetch', mockFetch(200, { nope: true }));
    const res = await loadBundleDetail('stark-review');
    expect(res.kind).toBe('degraded');
    if (res.kind === 'degraded') expect(res.reason).toBe('malformed');
  });

  it('degrades (unsupported-newer) on a skewed detail schemaVersion', async () => {
    vi.stubGlobal('fetch', mockFetch(200, { schemaVersion: 99, bundle: {}, artifacts: [] }));
    const res = await loadBundleDetail('stark-review');
    expect(res.kind).toBe('degraded');
    if (res.kind === 'degraded') expect(res.reason).toBe('unsupported-newer');
  });

  it('degrades (fetch-failed) on an HTTP error', async () => {
    vi.stubGlobal('fetch', mockFetch(503, null));
    const res = await loadBundleDetail('stark-review');
    expect(res.kind).toBe('degraded');
    if (res.kind === 'degraded') expect(res.reason).toBe('fetch-failed');
  });

  it('degrades (not throws) when the body json() rejects', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true, status: 200, json: async () => { throw new Error('boom'); },
    } as unknown as Response));
    const res = await loadBundleDetail('stark-review');
    expect(res.kind).toBe('degraded');
  });
});

describe('registryError', () => {
  it('maps reasons to user-facing copy mentioning the GitHub source', () => {
    expect(registryError('unsupported-newer')).toMatch(/github/i);
  });
});
