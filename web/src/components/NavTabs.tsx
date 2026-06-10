import { NavLink } from 'react-router-dom';

// Top-level section tabs, rendered inside each page's hero. Kept as NavLink so the active
// route gets aria-current + the .is-active class for styling, and deep links stay shareable
// under the HashRouter (see app.tsx for why hash routing).
const TABS: readonly { to: string; label: string }[] = [
  { to: '/', label: 'Browse' },
  { to: '/learn', label: 'MCP vs Skills' },
];

export function NavTabs(): JSX.Element {
  return (
    <nav className="nav-tabs" aria-label="Sections">
      {TABS.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          end={t.to === '/'}
          className={({ isActive }) => (isActive ? 'nav-tab is-active' : 'nav-tab')}
        >
          {t.label}
        </NavLink>
      ))}
    </nav>
  );
}
