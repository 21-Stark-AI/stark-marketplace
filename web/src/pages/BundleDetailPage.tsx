import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { loadBundleDetail, type DetailResult } from '../data/registry';
import { outputPathFor, type DetailArtifact, type Runtime } from '../types/registry';
import { SupportBadges } from '../components/SupportBadges';
import { InstallInstructions } from '../components/InstallInstructions';
import { DependencyGraph } from '../components/DependencyGraph';
import { DegradedPage } from './DegradedPage';

const RUNTIME_ORDER: readonly Runtime[] = ['claude', 'codex', 'gemini'];

// Display path per runtime = first engine-emitted output path (CC-3 derivation).
function OutputPaths({ artifact }: { readonly artifact: DetailArtifact }): JSX.Element {
  const rows = RUNTIME_ORDER
    .map((rt) => [rt, outputPathFor(artifact, rt)] as const)
    .filter((r): r is readonly [Runtime, string] => r[1] !== undefined);
  if (rows.length === 0) return <p className="outputs empty">No emitted outputs.</p>;
  return (
    <ul className="outputs">
      {rows.map(([rt, path]) => (
        <li key={rt}><span className="rt">{rt}</span>: <code>{path}</code></li>
      ))}
    </ul>
  );
}

const SOURCE_TREE = 'https://github.com/GetEvinced/stark-marketplace/tree/main/';

export function BundleDetailPage(): JSX.Element {
  const { name } = useParams<{ name: string }>();
  const [state, setState] = useState<DetailResult | 'loading'>('loading');

  useEffect(() => {
    if (!name) return;
    let active = true;
    void loadBundleDetail(name).then((r) => { if (active) setState(r); });
    return () => { active = false; };
  }, [name]);

  if (state === 'loading') return <main aria-busy="true">Loading bundle…</main>;
  if (state.kind === 'degraded') return <DegradedPage reason={state.reason} githubUrl={state.githubUrl} />;

  const { bundle, artifacts, dependencyClosure } = state.detail;
  return (
    <main>
      <p><Link to="/">← back to search</Link></p>
      <h1>{bundle.name}</h1>
      <p>{bundle.description}</p>
      <p>v{bundle.version} · {bundle.maturity} · {bundle.category}</p>
      {bundle.homepage ? <a href={bundle.homepage}>source on GitHub</a> : null}

      <h2>Install (whole bundle)</h2>
      <InstallInstructions bundle={bundle.name} support={{ claude: 'native', codex: 'native', gemini: 'native' }} />

      <h2>Artifacts</h2>
      {artifacts.map((a) => (
        <article key={a.name}>
          <h3>{a.name} <span className="type">{a.type}</span></h3>
          <p>{a.description}</p>
          <SupportBadges support={a.support} />
          <h4>Output paths</h4>
          <OutputPaths artifact={a} />
          <InstallInstructions bundle={bundle.name} artifact={a.name} type={a.type} support={a.support} />
          <a href={`${SOURCE_TREE}${a.sourcePath}`}>{a.sourcePath}</a>
        </article>
      ))}

      <h2>Dependencies</h2>
      <DependencyGraph edges={dependencyClosure} />
    </main>
  );
}
