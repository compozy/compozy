// The two reference diagrams of Graphic system → Diagrams, drawn in the wireframe language with
// @compozy/ui colour tokens. Agents are the live AgentAvatar face (ADR-005), placed in the SVG.
import { AgentFace } from "../art/pieces";
import type { AgentState } from "../art/roster";
import { icons, type IconName } from "./icons";

interface DiagramNode {
  x: number;
  y: number;
  w: number;
  label: string;
  gloss: string;
  /** glyph + its fill class */
  state?: [glyph: string, tone: string];
  hi?: boolean;
  icon?: IconName;
  agent?: [name: string, state: AgentState];
}

const H = 72;
const WORKING: [string, string] = ["●", "fill-brand-cyan"];
const NEEDS_YOU: [string, string] = ["◐", "fill-attn"];
const DONE: [string, string] = ["✓", "fill-success"];

const orch: DiagramNode = {
  x: 40,
  y: 184,
  w: 220,
  label: "checkout-lead",
  gloss: "Claude Code · lead",
  hi: true,
  agent: ["checkout-lead", "working"],
};
const kids: DiagramNode[] = [
  {
    x: 360,
    y: 56,
    w: 230,
    label: "payments-codex",
    gloss: "Codex · child session",
    state: WORKING,
    agent: ["payments-codex", "working"],
  },
  {
    x: 360,
    y: 184,
    w: 230,
    label: "docs-steward",
    gloss: "Gemini CLI · needs you",
    state: NEEDS_YOU,
    agent: ["docs-steward", "needs-you"],
  },
  {
    x: 360,
    y: 312,
    w: 230,
    label: "test-writer",
    gloss: "OpenCode · child session",
    state: DONE,
    agent: ["test-writer", "idle"],
  },
];
const TREE_IDS = ["7f2", "a19", "c40"];
const trees: DiagramNode[] = kids.map((k, i) => ({
  x: 690,
  y: k.y,
  w: 240,
  label: `wt/${k.label}`,
  gloss: `~/.compozy/worktrees/api-${TREE_IDS[i]}`,
  icon: "worktree",
}));
const loop: DiagramNode[] = [
  { x: 40, y: 120, w: 180, label: "goal", gloss: "tests in auth/ pass", icon: "task" },
  { x: 280, y: 120, w: 180, label: "act", gloss: "agent edits the code", icon: "agent" },
  { x: 520, y: 120, w: 180, label: "verify", gloss: "bun test auth", hi: true, icon: "approval" },
  { x: 760, y: 120, w: 160, label: "stop", gloss: "goal met · 06:31", state: DONE, icon: "loop" },
];

const COLUMNS = Array.from({ length: 13 }, (_, i) => i * 80);
const edge = "fill-none stroke-disabled";

function Node({ n, gid }: { n: DiagramNode; gid: string }) {
  const textX = n.x + (n.agent ? 68 : 40);
  return (
    <g>
      <rect
        x={n.x}
        y={n.y}
        width={n.w}
        height={H}
        rx="10"
        className={n.hi ? "fill-canvas" : "fill-canvas stroke-line-strong"}
        stroke={n.hi ? `url(#${gid})` : undefined}
        strokeWidth={n.hi ? 1.5 : 1}
      />
      {n.agent && (
        <foreignObject x={n.x + 14} y={n.y + 14} width="44" height="44" overflow="visible">
          <AgentFace name={n.agent[0]} state={n.agent[1]} size={44} />
        </foreignObject>
      )}
      {n.icon && (
        <g
          transform={`translate(${n.x + 16} ${n.y + 15}) scale(.6667)`}
          fill="none"
          className={n.hi ? "stroke-brand-cyan" : "stroke-muted-foreground"}
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          // Static markup from the icon set data (icons.ts).
          dangerouslySetInnerHTML={{ __html: icons[n.icon].path }}
        />
      )}
      <text x={textX} y={n.y + 30} className="fill-fg font-mono" fontSize="13" fontWeight="500">
        {n.label}
      </text>
      <text
        x={n.x + (n.agent ? 68 : 16)}
        y={n.y + 54}
        className="fill-muted-foreground font-sans"
        fontSize="12.5"
      >
        {n.gloss}
      </text>
      {n.state && (
        <text
          x={n.x + n.w - 14}
          y={n.y + 30}
          textAnchor="end"
          className={`${n.state[1]} font-mono`}
          fontSize="13"
        >
          {n.state[0]}
        </text>
      )}
    </g>
  );
}

function Arrow({ id }: { id: string }) {
  return (
    <marker
      id={id}
      viewBox="0 0 8 8"
      refX="7"
      refY="4"
      markerWidth="8"
      markerHeight="8"
      orient="auto"
    >
      <path d="M0 0L8 4L0 8" className="fill-none stroke-disabled" strokeWidth="1.2" />
    </marker>
  );
}

function Gradient({ id, x1, x2 }: { id: string; x1: number; x2: number }) {
  return (
    <linearGradient id={id} gradientUnits="userSpaceOnUse" x1={x1} y1="0" x2={x2} y2="0">
      <stop offset="0" style={{ stopColor: "var(--color-brand-volt)" }} />
      <stop offset=".5" style={{ stopColor: "var(--color-brand-sky)" }} />
      <stop offset="1" style={{ stopColor: "var(--color-brand-cyan)" }} />
    </linearGradient>
  );
}

function Columns({ h }: { h: number }) {
  return COLUMNS.map(x => <line key={x} x1={x} x2={x} y1="0" y2={h} className="stroke-elevated" />);
}

const caps = "fill-disabled font-mono";

export function DelegationDiagram() {
  return (
    <svg
      viewBox="0 0 960 440"
      className="block h-auto w-full"
      role="img"
      aria-label="An orchestrator agent delegates to three child sessions on Codex, Gemini CLI and OpenCode, each running in its own git worktree"
    >
      <defs>
        <Arrow id="a1" />
        <Gradient id="g1" x1={40} x2={260} />
      </defs>
      <Columns h={440} />
      {kids.map(k => (
        <path
          key={`d${k.label}`}
          d={`M260 220H300V${k.y + 36}H354`}
          className={edge}
          markerEnd="url(#a1)"
        />
      ))}
      {kids.map(k => (
        <path
          key={`r${k.label}`}
          d={`M590 ${k.y + 36}H684`}
          className={edge}
          markerEnd="url(#a1)"
        />
      ))}
      <text x="300" y="30" className={caps} fontSize="11" letterSpacing="1.2">
        DELEGATES
      </text>
      <text x="606" y="30" className={caps} fontSize="11" letterSpacing="1.2">
        RUNS IN
      </text>
      {[orch, ...kids, ...trees].map(n => (
        <Node key={n.label} n={n} gid="g1" />
      ))}
    </svg>
  );
}

export function LoopDiagram() {
  return (
    <svg
      viewBox="0 0 960 330"
      className="block h-auto w-full"
      role="img"
      aria-label="A loop: goal, act, verify; if the goal is not met it acts again, if met it stops"
    >
      <defs>
        <Arrow id="a2" />
        <Gradient id="g2" x1={520} x2={700} />
        <linearGradient id="g2r" gradientUnits="userSpaceOnUse" x1="610" y1="0" x2="370" y2="0">
          <stop offset="0" style={{ stopColor: "var(--color-brand-cyan)" }} />
          <stop offset="1" style={{ stopColor: "var(--color-brand-volt)" }} />
        </linearGradient>
      </defs>
      <Columns h={330} />
      <path d="M220 156H274" className={edge} markerEnd="url(#a2)" />
      <path d="M460 156H514" className={edge} markerEnd="url(#a2)" />
      <path d="M700 156H754" className={edge} markerEnd="url(#a2)" />
      <path
        d="M610 192V262H370V198"
        fill="none"
        stroke="url(#g2r)"
        strokeWidth="1.5"
        markerEnd="url(#a2)"
      />
      <text
        x="490"
        y="284"
        textAnchor="middle"
        className="fill-muted-foreground font-mono"
        fontSize="12"
      >
        not met → act again
      </text>
      <text
        x="727"
        y="146"
        textAnchor="middle"
        className="fill-muted-foreground font-mono"
        fontSize="11"
      >
        met
      </text>
      <text x="40" y="60" className={caps} fontSize="11" letterSpacing="1.2">
        LOOP · DETERMINISTIC STOP
      </text>
      {loop.map(n => (
        <Node key={n.label} n={n} gid="g2" />
      ))}
    </svg>
  );
}
