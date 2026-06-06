import { describe, it, expect } from 'vitest';
import { sum } from './smoke';

describe('vitest harness', () => {
  it('runs and imports app code', () => {
    expect(sum(2, 3)).toBe(5);
  });
});
