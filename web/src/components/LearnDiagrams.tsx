// Inline SVG schematics for the MCP-vs-Skills decision page. Brand hexes hard-coded
// (navy/blue/pink/yellow from the 21 Stark brand) so the drawings are self-contained and
// theme-stable. viewBox + width:100% keeps them crisp and responsive; aria-labelled for SR.

const NAVY = '#1b3050';
const BLUE = '#2578e0';
const PINK = '#f90078';
const LINE = '#dce6f2';
const INK = '#36506f';

/* How a Skill plugs in: static text folded into the model's context. */
export function SkillDiagram(): JSX.Element {
  return (
    <svg className="diagram" viewBox="0 0 340 180" role="img"
      aria-label="A markdown skill file is loaded into the agent's context, which the agent reads and follows.">
      {/* doc */}
      <rect x="18" y="48" width="74" height="92" rx="8" fill="#fff" stroke={PINK} strokeWidth="2" />
      <line x1="32" y1="70" x2="78" y2="70" stroke={PINK} strokeWidth="3" strokeLinecap="round" opacity="0.5" />
      <line x1="32" y1="84" x2="78" y2="84" stroke={INK} strokeWidth="2.5" strokeLinecap="round" opacity="0.35" />
      <line x1="32" y1="96" x2="78" y2="96" stroke={INK} strokeWidth="2.5" strokeLinecap="round" opacity="0.35" />
      <line x1="32" y1="108" x2="64" y2="108" stroke={INK} strokeWidth="2.5" strokeLinecap="round" opacity="0.35" />
      <text x="55" y="160" textAnchor="middle" className="diagram-cap">skill.md</text>
      {/* arrow */}
      <path d="M100 94 H150" stroke={NAVY} strokeWidth="2" fill="none" markerEnd="url(#arrowSkill)" />
      <text x="125" y="84" textAnchor="middle" className="diagram-note">load</text>
      {/* agent context */}
      <rect x="162" y="44" width="158" height="100" rx="12" fill="#f3f9fe" stroke={NAVY} strokeWidth="2" strokeDasharray="5 4" />
      <text x="241" y="68" textAnchor="middle" className="diagram-cap">agent context</text>
      <circle cx="241" cy="104" r="22" fill={NAVY} />
      <text x="241" y="109" textAnchor="middle" className="diagram-glyph">★</text>
      <defs>
        <marker id="arrowSkill" markerWidth="9" markerHeight="9" refX="7" refY="4.5" orient="auto">
          <path d="M0 0 L9 4.5 L0 9 z" fill={NAVY} />
        </marker>
      </defs>
    </svg>
  );
}

/* How an MCP server plugs in: a runtime round-trip out to an external system. */
export function McpDiagram(): JSX.Element {
  return (
    <svg className="diagram" viewBox="0 0 340 180" role="img"
      aria-label="The agent calls an MCP server at runtime; the server executes against an external system and returns live results.">
      {/* agent */}
      <rect x="14" y="62" width="78" height="56" rx="10" fill={NAVY} />
      <text x="53" y="94" textAnchor="middle" className="diagram-glyph">★</text>
      <text x="53" y="138" textAnchor="middle" className="diagram-cap">agent</text>
      {/* arrows agent <-> server */}
      <path d="M96 80 H150" stroke={BLUE} strokeWidth="2" fill="none" markerEnd="url(#arrowMcp)" />
      <path d="M150 100 H96" stroke={BLUE} strokeWidth="2" fill="none" markerEnd="url(#arrowMcpBack)" opacity="0.55" />
      <text x="123" y="72" textAnchor="middle" className="diagram-note">call</text>
      {/* server */}
      <rect x="152" y="60" width="78" height="60" rx="10" fill="#fff" stroke={BLUE} strokeWidth="2" />
      <text x="191" y="86" textAnchor="middle" className="diagram-cap" fill={BLUE}>MCP</text>
      <text x="191" y="100" textAnchor="middle" className="diagram-cap" fill={BLUE}>server</text>
      <text x="191" y="138" textAnchor="middle" className="diagram-cap">runs code</text>
      {/* arrows server <-> external */}
      <path d="M234 80 H286" stroke={BLUE} strokeWidth="2" fill="none" markerEnd="url(#arrowMcp)" />
      <path d="M286 100 H234" stroke={BLUE} strokeWidth="2" fill="none" markerEnd="url(#arrowMcpBack)" opacity="0.55" />
      {/* external system (cylinder) */}
      <g>
        <path d="M290 66 h36 v44 a18 7 0 0 1 -36 0 z" fill="#f3f9fe" stroke={NAVY} strokeWidth="2" />
        <ellipse cx="308" cy="66" rx="18" ry="7" fill="#fff" stroke={NAVY} strokeWidth="2" />
        <text x="308" y="138" textAnchor="middle" className="diagram-cap">API / DB</text>
      </g>
      <defs>
        <marker id="arrowMcp" markerWidth="9" markerHeight="9" refX="7" refY="4.5" orient="auto">
          <path d="M0 0 L9 4.5 L0 9 z" fill={BLUE} />
        </marker>
        <marker id="arrowMcpBack" markerWidth="9" markerHeight="9" refX="7" refY="4.5" orient="auto">
          <path d="M0 0 L9 4.5 L0 9 z" fill={BLUE} />
        </marker>
      </defs>
    </svg>
  );
}

/* The chooser: one runtime question splitting into MCP / Skill, converging on "pair them". */
export function DecisionDiagram(): JSX.Element {
  return (
    <svg className="diagram diagram--wide" viewBox="0 0 720 330" role="img"
      aria-label="Decision guide: if the agent must act on or fetch from an external system at runtime, choose MCP. If it needs know-how or a workflow, choose a Skill. For hard cases, pair an MCP server with skills that teach the agent how to wield it.">
      {/* top question */}
      <rect x="240" y="14" width="240" height="56" rx="12" fill={NAVY} />
      <text x="360" y="40" textAnchor="middle" className="diagram-q">What must the agent gain</text>
      <text x="360" y="58" textAnchor="middle" className="diagram-q">to do this job?</text>
      {/* connectors */}
      <path d="M300 70 C220 110 170 110 140 138" stroke={LINE} strokeWidth="2.5" fill="none" />
      <path d="M420 70 C500 110 550 110 580 138" stroke={LINE} strokeWidth="2.5" fill="none" />
      {/* MCP branch */}
      <rect x="30" y="140" width="240" height="92" rx="12" fill="#fff" stroke={BLUE} strokeWidth="2.5" />
      <text x="150" y="168" textAnchor="middle" className="diagram-branch" fill={BLUE}>To ACT on / FETCH from</text>
      <text x="150" y="188" textAnchor="middle" className="diagram-branch" fill={BLUE}>an external system, live</text>
      <text x="150" y="216" textAnchor="middle" className="diagram-verdict" fill={BLUE}>→ MCP server</text>
      {/* Skill branch */}
      <rect x="450" y="140" width="240" height="92" rx="12" fill="#fff" stroke={PINK} strokeWidth="2.5" />
      <text x="570" y="168" textAnchor="middle" className="diagram-branch" fill={PINK}>Know-how, a workflow,</text>
      <text x="570" y="188" textAnchor="middle" className="diagram-branch" fill={PINK}>conventions to follow</text>
      <text x="570" y="216" textAnchor="middle" className="diagram-verdict" fill={PINK}>→ Skill</text>
      {/* converge */}
      <path d="M150 232 C150 280 300 280 350 286" stroke={LINE} strokeWidth="2.5" fill="none" />
      <path d="M570 232 C570 280 420 280 370 286" stroke={LINE} strokeWidth="2.5" fill="none" />
      <rect x="210" y="278" width="300" height="44" rx="22" fill="#f3f9fe" stroke={NAVY} strokeWidth="2" strokeDasharray="5 4" />
      <text x="360" y="305" textAnchor="middle" className="diagram-both">Both? Capability + the playbook to wield it</text>
    </svg>
  );
}
