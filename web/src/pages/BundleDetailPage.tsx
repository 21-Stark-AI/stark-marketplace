import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { loadBundleDetail, type DetailResult } from '../data/registry';
import {
  outputPathFor,
  type DetailArtifact,
  type Runtime,
  type SupportLevel,
  type SupportMatrix,
} from '../types/registry';
import { SupportBadges } from '../components/SupportBadges';
import { InstallInstructions } from '../components/InstallInstructions';
import { DependencyGraph } from '../components/DependencyGraph';
import { edgesFromArtifacts } from '../graph/deps';
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

// Aggregate whole-bundle support from the artifacts (the engine emits no bundle-level support):
// a runtime is shown only if some artifact targets it; native only if EVERY targeting artifact
// is native, otherwise emulated. This keeps the bundle install block honest vs. per-artifact badges.
function aggregateBundleSupport(artifacts: readonly DetailArtifact[]): SupportMatrix {
  const out: Partial<Record<Runtime, SupportLevel>> = {};
  for (const rt of RUNTIME_ORDER) {
    let any = false;
    let allNative = true;
    for (const a of artifacts) {
      const lvl = a.support[rt];
      if (lvl === 'native' || lvl === 'emulated') {
        any = true;
        if (lvl !== 'native') allNative = false;
      }
    }
    if (any) out[rt] = allNative ? 'native' : 'emulated';
  }
  return out;
}

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

  const { bundle, artifacts } = state.detail;
  const edges = edgesFromArtifacts(bundle.name, artifacts);
  return (
    <main>
      <p><Link to="/">← back to search</Link></p>
      <h1>{bundle.name}</h1>
      {bundle.description ? <p>{bundle.description}</p> : null}
      <p>v{bundle.version}{bundle.maturity ? ` · ${bundle.maturity}` : ''}{bundle.category ? ` · ${bundle.category}` : ''}</p>
      {bundle.homepage ? <a href={bundle.homepage}>source on GitHub</a> : null}

      <h2>Install (whole bundle)</h2>
      <InstallInstructions bundle={bundle.name} support={aggregateBundleSupport(artifacts)} headingLevel={3} />

      <h2>Artifacts</h2>
      {artifacts.map((a) => (
        <article key={a.name}>
          <h3>{a.name} <span className="type">{a.type}</span></h3>
          {a.description ? <p>{a.description}</p> : null}
          <SupportBadges support={a.support} />
          <h4>Output paths</h4>
          <OutputPaths artifact={a} />
          <InstallInstructions bundle={bundle.name} artifact={a.name} type={a.type} support={a.support} headingLevel={4} />
          {bundle.homepage ? <a href={bundle.homepage}>{a.name} source on GitHub</a> : null}
        </article>
      ))}

      <h2>Dependencies</h2>
      <DependencyGraph edges={edges} />
    </main>
  );
}
