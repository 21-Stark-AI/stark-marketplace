import { useEffect, useState } from 'react';
// HashRouter (not BrowserRouter): the SPA is served from a dumb gated-static origin with a
// relative asset base, so history-API deep links would 404 their assets and need an origin
// rewrite rule. Hash routing keeps every route on the root document — shareable deep links work
// with no server config (spec §10 deep links / docs/web-hosting.md).
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom';
import { loadIndex, type IndexResult } from './data/registry';
import { SearchPage } from './pages/SearchPage';
import { DegradedPage } from './pages/DegradedPage';
import { BundleDetailPage } from './pages/BundleDetailPage';
import { LearnPage } from './pages/LearnPage';

export function App(): JSX.Element {
  const [state, setState] = useState<IndexResult | 'loading'>('loading');
  useEffect(() => {
    let active = true;
    void loadIndex().then((r) => { if (active) setState(r); });
    return () => { active = false; };
  }, []);

  if (state === 'loading') return <main aria-busy="true">Loading registry…</main>;
  if (state.kind === 'degraded') return <DegradedPage reason={state.reason} githubUrl={state.githubUrl} />;

  return (
    <HashRouter>
      <Routes>
        <Route path="/" element={<SearchPage index={state.index} />} />
        <Route path="/learn" element={<LearnPage />} />
        <Route path="/bundle/:name" element={<BundleDetailPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </HashRouter>
  );
}
