import type { Runtime, SupportLevel } from '../types/registry';
import { installSnippets } from '../install/snippets';

interface Props {
  readonly bundle: string;
  readonly artifact?: string;
  readonly type?: string;
  readonly support: Partial<Record<Runtime, SupportLevel>>;
  // Heading level for each per-runtime section so the page's heading outline never skips
  // (h2→h3 for the whole-bundle block, h3→h4 under an artifact). Defaults to 4.
  readonly headingLevel?: 3 | 4;
}

const RUNTIMES: readonly Runtime[] = ['claude', 'codex', 'gemini'];

export function InstallInstructions({ bundle, artifact, type, support, headingLevel = 4 }: Props): JSX.Element {
  const Heading = headingLevel === 3 ? 'h3' : 'h4';
  return (
    <div className="install">
      {RUNTIMES.filter((rt) => support[rt] !== undefined).map((rt) => {
        const level = support[rt] as SupportLevel;
        const snip = installSnippets({ bundle, artifact, type, runtime: rt, support: level });
        return (
          <section key={rt}>
            <Heading>{rt} ({snip.surface})</Heading>
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
