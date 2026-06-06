import type { DegradeReason } from '../data/registry';
import { registryError } from '../data/registry';

export function DegradedPage({ reason, githubUrl }: { readonly reason: DegradeReason; readonly githubUrl: string }): JSX.Element {
  return (
    <main role="alert">
      <h1>Registry unavailable</h1>
      <p>{registryError(reason)}</p>
      <a href={githubUrl}>Open the source on GitHub</a>
    </main>
  );
}
