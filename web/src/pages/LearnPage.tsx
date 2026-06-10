import { Link } from 'react-router-dom';
import { NavTabs } from '../components/NavTabs';
import { SkillDiagram, McpDiagram, DecisionDiagram } from '../components/LearnDiagrams';

// Decision-oriented explainer. Assumes the reader already knows what MCP and Skills are —
// the job here is "which fits which use case". Content only; no registry data needed.

type Verdict = 'mcp' | 'skill' | 'both';

interface UseCase {
  readonly need: string;
  readonly verdict: Verdict;
  readonly why: string;
}

const CASES: readonly UseCase[] = [
  { need: 'Read live data from an API, DB, or SaaS at call time', verdict: 'mcp', why: 'Needs a process to execute and return fresh results — text can’t fetch.' },
  { need: 'Take an action in another system (open a PR, post to Slack)', verdict: 'mcp', why: 'Side effects require running code and credentials, not instructions.' },
  { need: 'Deterministic, structured tool I/O the model can chain', verdict: 'mcp', why: 'Typed tool schemas give reliable inputs/outputs the agent can compose.' },
  { need: 'Teach a multi-step workflow or review playbook', verdict: 'skill', why: 'Pure procedure the model follows — no execution, no server to run.' },
  { need: 'Encode house conventions, style, or guardrails', verdict: 'skill', why: 'Behavior-shaping guidance; ships as text and versions in git.' },
  { need: 'Keep token cost low until the capability is actually needed', verdict: 'skill', why: 'Skills load on demand; MCP tool defs sit in context all session.' },
  { need: 'Zero ops — nothing to host, authenticate, or monitor', verdict: 'skill', why: 'A skill is a file. An MCP server is infra you have to keep alive.' },
  { need: 'Give a capability AND teach the agent to use it well', verdict: 'both', why: 'MCP supplies the hands; a skill supplies the judgement and sequence.' },
];

const VERDICT_LABEL: Record<Verdict, string> = { mcp: 'MCP', skill: 'Skill', both: 'Both' };

export function LearnPage(): JSX.Element {
  return (
    <main>
      <header className="hero hero--compact">
        <p className="hero-kicker">field guide</p>
        <h1>
          MCP <span className="accent">or</span> Skill?
        </h1>
        <p className="hero-sub">
          Both extend the agent — but along different axes. Here’s how to pick the right one
          for the job, and when to ship both.
        </p>
        <NavTabs />
      </header>

      <div className="content learn">
        <h2>The core split</h2>
        <p className="learn-lead">
          An <b>MCP server</b> gives the agent <em>reach</em> — a live connection that runs code
          and acts on the outside world. A <b>Skill</b> gives the agent <em>judgement</em> — text
          it reads to know how to approach the work. One adds capability; the other adds know-how.
        </p>

        <section className="learn-pair">
          <article className="learn-card learn-card--skill">
            <p className="learn-tag learn-tag--skill">Skill · know-how</p>
            <SkillDiagram />
            <p className="learn-foot">Text into context. Nothing runs. Loads when relevant.</p>
          </article>
          <article className="learn-card learn-card--mcp">
            <p className="learn-tag">MCP · capability</p>
            <McpDiagram />
            <p className="learn-foot">A runtime round-trip. Executes, returns live data.</p>
          </article>
        </section>

        <h2>How to choose</h2>
        <DecisionDiagram />

        <h2>Use case → pick</h2>
        <div className="learn-table-wrap">
          <table className="learn-table">
            <thead>
              <tr>
                <th scope="col">If you need to…</th>
                <th scope="col">Reach for</th>
                <th scope="col">Why</th>
              </tr>
            </thead>
            <tbody>
              {CASES.map((c) => (
                <tr key={c.need}>
                  <th scope="row">{c.need}</th>
                  <td>
                    <span className={`verdict verdict--${c.verdict}`}>{VERDICT_LABEL[c.verdict]}</span>
                  </td>
                  <td>{c.why}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <h2>Rule of thumb</h2>
        <p className="learn-rule">
          Pick <b>MCP</b> when the bottleneck is <b>access</b> — the agent literally can’t reach
          the thing. Pick a <b>Skill</b> when the bottleneck is <b>knowledge</b> — it can reach
          the thing but doesn’t know how to do it well. The strongest bundles pair them: an MCP
          server for the hands, skills for the playbook.
        </p>

        <p className="learn-back">
          <Link to="/">← Browse the catalog</Link>
        </p>
      </div>
    </main>
  );
}
