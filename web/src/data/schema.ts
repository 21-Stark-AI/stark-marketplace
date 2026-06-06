// The schemaVersion this SPA build understands. Bumped only on genuine breaks.
// Backward compat by convention: additive fields within a version (unknown fields
// ignored); older indexes down to SUPPORTED_SCHEMA - MIN_BACK still render (N-1).
export const SUPPORTED_SCHEMA = 1;
export const MIN_BACK = 1; // accept versions [SUPPORTED_SCHEMA - MIN_BACK, SUPPORTED_SCHEMA]

export type Negotiation =
  | { readonly ok: true; readonly indexVersion: number }
  | {
      readonly ok: false;
      readonly reason: 'unsupported-newer' | 'unsupported-older' | 'invalid';
      readonly indexVersion: number;
    };

export function negotiate(indexVersion: number): Negotiation {
  if (!Number.isInteger(indexVersion)) {
    return { ok: false, reason: 'invalid', indexVersion };
  }
  if (indexVersion > SUPPORTED_SCHEMA) {
    return { ok: false, reason: 'unsupported-newer', indexVersion };
  }
  if (indexVersion < SUPPORTED_SCHEMA - MIN_BACK) {
    return { ok: false, reason: 'unsupported-older', indexVersion };
  }
  return { ok: true, indexVersion };
}
