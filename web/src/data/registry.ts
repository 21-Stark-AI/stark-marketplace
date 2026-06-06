import {
  isLeanIndex,
  isBundleDetail,
  type LeanIndex,
  type BundleDetail,
} from '../types/registry';
import { negotiate, type Negotiation } from './schema';

const GITHUB_SOURCE = 'https://github.com/GetEvinced/stark-marketplace';

export type DegradeReason =
  | Exclude<Extract<Negotiation, { ok: false }>['reason'], never>
  | 'fetch-failed'
  | 'malformed';

export type IndexResult =
  | { readonly kind: 'ok'; readonly index: LeanIndex }
  | { readonly kind: 'degraded'; readonly reason: DegradeReason; readonly githubUrl: string };

export type DetailResult =
  | { readonly kind: 'ok'; readonly detail: BundleDetail }
  | { readonly kind: 'degraded'; readonly reason: DegradeReason; readonly githubUrl: string };

const degradedIndex = (reason: DegradeReason): IndexResult => ({
  kind: 'degraded',
  reason,
  githubUrl: GITHUB_SOURCE,
});

async function fetchJson(url: string): Promise<unknown | undefined> {
  try {
    const resp = await fetch(url, { credentials: 'same-origin' });
    if (!resp.ok) return undefined;
    return (await resp.json()) as unknown;
  } catch {
    return undefined;
  }
}

export async function loadIndex(url = './index.json'): Promise<IndexResult> {
  const raw = await fetchJson(url);
  if (raw === undefined) return degradedIndex('fetch-failed');
  if (!isLeanIndex(raw)) return degradedIndex('malformed');
  const neg = negotiate(raw.schemaVersion);
  if (!neg.ok) return degradedIndex(neg.reason);
  return { kind: 'ok', index: raw };
}

export async function loadBundleDetail(bundle: string): Promise<DetailResult> {
  const url = `./bundles/${encodeURIComponent(bundle)}.json`;
  const raw = await fetchJson(url);
  if (raw === undefined) return { kind: 'degraded', reason: 'fetch-failed', githubUrl: GITHUB_SOURCE };
  if (!isBundleDetail(raw)) return { kind: 'degraded', reason: 'malformed', githubUrl: GITHUB_SOURCE };
  const neg = negotiate(raw.schemaVersion);
  if (!neg.ok) return { kind: 'degraded', reason: neg.reason, githubUrl: GITHUB_SOURCE };
  return { kind: 'ok', detail: raw };
}

export function registryError(reason: DegradeReason): string {
  switch (reason) {
    case 'unsupported-newer':
      return `The registry index is newer than this app build. The registry is updating — meanwhile, browse the source on GitHub: ${GITHUB_SOURCE}`;
    case 'unsupported-older':
      return `This app build expects a newer registry index. Browse the source on GitHub: ${GITHUB_SOURCE}`;
    case 'fetch-failed':
      return `Couldn't reach the registry data (you may need to re-authenticate). Source on GitHub: ${GITHUB_SOURCE}`;
    case 'malformed':
    case 'invalid':
      return `The registry index couldn't be read. Browse the source on GitHub: ${GITHUB_SOURCE}`;
  }
}
