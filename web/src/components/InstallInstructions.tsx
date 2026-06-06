import type { Runtime, SupportLevel } from '../types/registry';
import { installSnippets } from '../install/snippets';

interface Props {
  readonly bundle: string;
  readonly artifact?: string;
  readonly type?: string;
  readonly support: Partial<Record<Runtime, SupportLevel>>;
}

const RUNTIMES: readonly Runtime[] = ['claude', 'codex', 'gemini'];

export function InstallInstructions({ bundle, artifact, type, support }: Props): JSX.Element {
  return (
    <div className="install">
      {RUNTIMES.filter((rt) => support[rt] !== undefined).map((rt) => {
        const level = support[rt] as SupportLevel;
        const snip = installSnippets({ bundle, artifact, type, runtime: rt, support: level });
        return (
          <section key={rt}>
            <h4>{rt} ({snip.surface})</h4>
            {snip.commands.length > 0 ? (
              <pre><code>{snip.commands.join('\n')}</code></pre>
            ) : null}
            {snip.note ? <p className="note">{snip.note}</p> : null}
          </section>
        );
      })}
    </div>
  );
}
