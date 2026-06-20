import type { LeanIndex } from '../types/registry';

// Shows the engine provenance from index.json (adapter versions, generated timestamp)
// plus a link to the latest signed build manifest on the GitHub release. The cryptographic
// verification itself is a CLI concern (`stark verify-manifest`); the SPA's job is to
// surface the trust anchor so a curious user can pivot to verification with one click.
const RELEASE_LATEST = 'https://github.com/21-Stark-AI/stark-marketplace/releases/latest';

export function ProvenanceBadge({ index }: { readonly index: LeanIndex }): JSX.Element | null {
  const av = index.generatedBy?.adapterVersions;
  const at = index.generatedAt;
  if (!av && !at) return null;

  return (
    <aside className="provenance" aria-label="Build provenance">
      {av && (
        <span className="provenance-adapters">
          {(['claude', 'codex', 'gemini'] as const)
            .map((rt) => av[rt])
            .filter((v): v is string => Boolean(v))
            .join(' · ')}
        </span>
      )}
      {at && <time className="provenance-time" dateTime={at}>{at}</time>}
      <a className="provenance-manifest" href={RELEASE_LATEST} rel="noopener noreferrer" target="_blank">
        Verify signed manifest →
      </a>
    </aside>
  );
}
