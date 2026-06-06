import { describe, it, expect } from 'vitest';
import indexFixture from '../__fixtures__/index.json';
import detailFixture from '../__fixtures__/bundles/stark-review.json';
import { isLeanIndex, isBundleDetail } from './registry';

describe('registry type guards', () => {
  it('accepts the lean index fixture', () => {
    expect(isLeanIndex(indexFixture)).toBe(true);
  });
  it('accepts the bundle detail fixture', () => {
    expect(isBundleDetail(detailFixture)).toBe(true);
  });
  it('ignores unknown extra fields (forward compat)', () => {
    const withExtra = { ...(indexFixture as object), futureField: 42 };
    expect(isLeanIndex(withExtra)).toBe(true);
  });
  it('rejects a record missing schemaVersion', () => {
    expect(isLeanIndex({ artifacts: [] })).toBe(false);
  });
});
