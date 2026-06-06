import { describe, it, expect } from 'vitest';
import { installSnippets } from './snippets';

describe('installSnippets', () => {
  it('claude native uses /plugin marketplace add + /plugin install', () => {
    const s = installSnippets({ bundle: 'stark-review', runtime: 'claude', support: 'native' });
    expect(s.surface).toBe('claude-code');
    expect(s.commands.join('\n')).toContain('/plugin marketplace add GetEvinced/stark-marketplace');
    expect(s.commands.join('\n')).toContain('/plugin install stark-review');
  });

  it('codex uses the stark CLI with --runtime codex', () => {
    const s = installSnippets({ bundle: 'stark-review', runtime: 'codex', support: 'native' });
    expect(s.commands.some((c) => c.includes('stark install stark-review --runtime codex'))).toBe(true);
  });

  it('gemini emulated carries a verify note', () => {
    const s = installSnippets({ bundle: 'stark-review', runtime: 'gemini', support: 'emulated' });
    expect(s.commands.some((c) => c.includes('--runtime gemini'))).toBe(true);
    expect(s.note).toMatch(/emulated/i);
  });

  it('per-artifact install scopes to bundle/artifact', () => {
    const s = installSnippets({ bundle: 'stark-review', artifact: 'review', runtime: 'codex', support: 'native' });
    expect(s.commands.some((c) => c.includes('stark install stark-review/review --runtime codex'))).toBe(true);
  });

  it('unsupported yields no commands and an explanatory note', () => {
    const s = installSnippets({ bundle: 'b', runtime: 'gemini', support: 'unsupported' });
    expect(s.commands).toEqual([]);
    expect(s.note).toMatch(/not supported/i);
  });
});
