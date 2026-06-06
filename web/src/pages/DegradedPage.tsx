import type { DegradeReason } from '../data/registry';
import { registryError } from '../data/registry';

export function DegradedPage({ reason, githubUrl }: { readonly reason: DegradeReason; readonly githubUrl: string }): JSX.Element {
  // Keep the `main` landmark (so heading/landmark navigation works) and scope the live region
  // to just the status sentence — role="alert" on the whole page would drop the landmark and
  // assertively announce the entire document including the heading and link.
  return (
    <main>
      <h1>Registry unavailable</h1>
      <p role="status">{registryError(reason)}</p>
      <a href={githubUrl}>Open the source on GitHub</a>
    </main>
  );
}
