import type { SupportMatrix, Runtime } from '../types/registry';

const ORDER: readonly Runtime[] = ['claude', 'codex', 'gemini'];

export function SupportBadges({ support }: { readonly support: SupportMatrix }): JSX.Element {
  return (
    <span className="support-badges">
      {ORDER.filter((rt) => support[rt] !== undefined).map((rt) => (
        <span key={rt} className="badge" data-support={support[rt]}>
          {rt}: {support[rt]}
        </span>
      ))}
    </span>
  );
}
