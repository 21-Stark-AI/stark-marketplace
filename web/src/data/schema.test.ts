import { describe, it, expect } from 'vitest';
import skewed from '../__fixtures__/index.skewed.json';
import current from '../__fixtures__/index.json';
import { SUPPORTED_SCHEMA, negotiate } from './schema';

describe('schema negotiation', () => {
  it('accepts the current supported schemaVersion', () => {
    const r = negotiate((current as { schemaVersion: number }).schemaVersion);
    expect(r.ok).toBe(true);
  });
  it('accepts N-1 (older index, newer SPA)', () => {
    const r = negotiate(SUPPORTED_SCHEMA - 1);
    expect(r.ok).toBe(true);
  });
  it('degrades on a future schemaVersion (newer index, older SPA)', () => {
    const r = negotiate((skewed as { schemaVersion: number }).schemaVersion);
    expect(r.ok).toBe(false);
    if (!r.ok) {
      expect(r.reason).toBe('unsupported-newer');
      expect(r.indexVersion).toBe(99);
    }
  });
  it('degrades on a missing/NaN version', () => {
    const r = negotiate(Number.NaN);
    expect(r.ok).toBe(false);
  });
});
