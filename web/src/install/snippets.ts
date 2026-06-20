import type { Runtime, SupportLevel } from '../types/registry';

export interface InstallTarget {
  readonly bundle: string;
  // explicit `| undefined` so callers may pass through possibly-undefined values under
  // exactOptionalPropertyTypes (the SPA threads optional artifact/type from props).
  readonly artifact?: string | undefined;
  readonly type?: string | undefined;
  readonly runtime: Runtime;
  readonly support: SupportLevel;
}

export interface InstallSnippet {
  readonly surface: 'claude-code' | 'stark-cli';
  readonly commands: readonly string[];
  readonly note?: string;
}

const MARKETPLACE = '21-Stark-AI/stark-marketplace';

const target = (t: InstallTarget): string =>
  t.artifact ? `${t.bundle}/${t.artifact}` : t.bundle;

export function installSnippets(t: InstallTarget): InstallSnippet {
  if (t.support === 'unsupported') {
    return { surface: 'stark-cli', commands: [], note: `${t.type ?? 'This artifact'} is not supported on ${t.runtime}.` };
  }
  if (t.runtime === 'claude') {
    // Claude Code installs natively from the committed dist tree (spec §8).
    const cmds = [`/plugin marketplace add ${MARKETPLACE}`, `/plugin install ${t.bundle}`];
    return { surface: 'claude-code', commands: cmds };
  }
  // Codex + Gemini install via the stark CLI (spec §9).
  const cmd = `stark install ${target(t)} --runtime ${t.runtime}`;
  const note =
    t.support === 'emulated'
      ? `Emulated on ${t.runtime}: derived shape; may not auto-activate — verify after install.`
      : undefined;
  return note === undefined
    ? { surface: 'stark-cli', commands: [cmd] }
    : { surface: 'stark-cli', commands: [cmd], note };
}
