// Copy for the Visual system chapter. Components only map over these arrays.
import type { AgentState } from "../roster";
import type { VisualName } from "../art/visuals";

export const principleFrames: [title: string, text: string, visual: VisualName][] = [
  [
    "01 · Many harnesses",
    "Claude Code, Codex, Gemini, OpenCode: work starts everywhere and arrives scattered.",
    "converge",
  ],
  [
    "02 · One control plane",
    "One desktop where you see and command all of them: faces, board, approvals.",
    "desktop",
  ],
  [
    "03 · Autonomous",
    "Sessions hand work to each other and check in on their own. You step in when they need you.",
    "orchestrate",
  ],
];

export const familyUse: [piece: string, family: string][] = [
  ["Brand ad, banner, poster", "Atmosphere: swarm, converge, block, terrain"],
  ["Quiet background behind long text", "Atmosphere: cone, horizon, field at 40–70% opacity"],
  ["Product, release, feature", "Live: desktop, agents, board, orchestrate, approval"],
  ["Task detail, changelog", "Live: sessions, stack"],
  ["Numbers, usage, reports", "Live: lanes, bars"],
  ["Paper (light) surfaces", "Live with theme light only. Rasters never go on paper."],
];

export const atmosphereUse: Record<string, string> = {
  swarm: "Hero, covers, X/LinkedIn banners, brand ads",
  converge: 'Launches, "how it works", slide titles',
  block: "Product launch, posters, square ads",
  cone: "Quiet backgrounds, story frames, end cards",
  horizon: "Wide banners, footers, call backgrounds",
  terrain: "Long banners, rollups, wallpapers",
  field: "Dense layouts, badges, empty states",
};

/** where each Live family comes from (ADR-005) */
export const liveSource: Record<string, string> = {
  product: "Capture of the real desktop components, current theme",
  diagram: "Diagram built from product tokens",
};

/** state shown on each roster face, in roster order */
export const rosterStates: AgentState[] = [
  "working",
  "needs-you",
  "idle",
  "working",
  "paused",
  "idle",
  "failed",
];

export const faceStates: [state: AgentState, label: string, text: string][] = [
  ["working", "Working", "Thinking face, cyan dot. The agent is doing the work."],
  ["needs-you", "Needs you", "Surprised face, amber triangle. A decision is waiting."],
  ["idle", "Idle", "Resting face, grey dot. Checked in, nothing to report."],
  ["paused", "Paused", "Sleepy face. Paused by you or by schedule."],
  ["failed", "Failed", "Sad face, red dot. The run stopped and says why."],
];

/** hex is the displayed value; fill reads the @compozy/ui token that carries it */
export const lightSwatches: { hex: string; name: string; text: string; fill: string }[] = [
  {
    hex: "#000000",
    name: "Raster ground",
    text: "Atmosphere blacks; reads as ink next to #0a0a0a",
    fill: "var(--color-black)",
  },
  { hex: "#0a0a0a", name: "Ink", text: "Page and template ground", fill: "var(--color-rail)" },
  {
    hex: "#2F5BFF",
    name: "Volt",
    text: "Live gradient start; buttons and small marks",
    fill: "var(--color-brand-volt)",
  },
  {
    hex: "#3DF2E0",
    name: "Cyan",
    text: "The brightest point: what is live right now",
    fill: "var(--color-brand-cyan)",
  },
  { hex: "#ececec", name: "Fg", text: "All type on visuals", fill: "var(--color-fg)" },
];

export const pairingLevels: {
  title: string;
  headline: string;
  visual: VisualName;
  x?: number;
  note: string;
}[] = [
  {
    title: "Loud · brand",
    headline: "Agents that keep running.",
    visual: "block",
    x: 0.72,
    note: "Atmosphere fills two thirds.",
  },
  {
    title: "Medium · release",
    headline: "Release 0.7: loops with approvals.",
    visual: "stack",
    x: 0.68,
    note: "Live product state, one third to half.",
  },
  {
    title: "Quiet · docs",
    headline: "Configure your first loop.",
    visual: "horizon",
    note: "Atmosphere at 60%, behind long text.",
  },
];

export type DontKind = "text-on-light" | "hue" | "two" | "paper" | "ring" | "fake";
export const donts: [title: string, text: string, visual: VisualName, kind: DontKind][] = [
  [
    "Text on the light",
    "Headlines never sit on the brightest area. Move the focal point or add the scrim.",
    "swarm",
    "text-on-light",
  ],
  [
    "Recolour",
    "No hue shifts, filters or duotones. Purple, orange and green are not in the system.",
    "cone",
    "hue",
  ],
  [
    "Two visuals",
    "One visual per piece. Two images or two families fight for the same eye.",
    "terrain",
    "two",
  ],
  [
    "Rasters on paper",
    "Atmosphere is dark only. On paper, use Live with theme light.",
    "field",
    "paper",
  ],
  [
    "Shapes and rings",
    "No loops, rings, orbits or symbol patterns as decoration. The symbol is the only shape.",
    "horizon",
    "ring",
  ],
  [
    "Fake product",
    'Live visuals show states the product has. No invented features, no lorem ipsum, no fake metrics without "sample".',
    "sessions",
    "fake",
  ],
];
