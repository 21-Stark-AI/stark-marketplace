import { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { loadIndex, type IndexResult } from './data/registry';
import { SearchPage } from './pages/SearchPage';
import { DegradedPage } from './pages/DegradedPage';
import { BundleDetailPage } from './pages/BundleDetailPage';

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
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<SearchPage index={state.index} />} />
        <Route path="/bundle/:name" element={<BundleDetailPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
