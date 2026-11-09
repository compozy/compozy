// Copy for chapter 05, Product. Components only map over these arrays. Hex strings are the
// documented spec values (shown as text and fed to the contrast checks); fills that have a
// @compozy/ui token read it by name.
import type { IconName } from "../graphics/icons";

const v = (token: string) => `var(${token})`;

export const principles: { n: string; title: string; body: string; rule: string }[] = [
  {
    n: "01",
    title: "The desktop is the product",
    body: "People spend their day in one window that shows every agent and every harness. The desktop is where design effort goes; the CLI and the API are secondary surfaces for scripts and power users.",
    rule: "Every brand decision is tested on the desktop first. The terminal palette follows it, never leads.",
  },
  {
    n: "02",
    title: "Autonomy is visible",
    body: "Always-on agents check in on their own and call you only when needed. The interface shows who is working, who is resting and who is waiting for a decision, without a click.",
    rule: "Face + state mark + word + age on every agent row, card and notification. Colour alone never carries state.",
  },
  {
    n: "03",
    title: "One control plane, many harnesses",
    body: "Claude Code, Codex, Gemini CLI, OpenCode and Cursor appear side by side. The harness is a label on the agent, never a separate product or a different look.",
    rule: "Harness name as a chip or mono label, in neutral grey. No per-harness brand colour on rows.",
  },
  {
    n: "04",
    title: "The exception is loud",
    body: "When something needs a person, it is the only coloured thing in its region. Needs you rises to the top, the dock badge counts it, the menu-bar icon changes.",
    rule: "One amber element per view region. If everything is highlighted, nothing is.",
  },
  {
    n: "05",
    title: "Dense, not crowded",
    body: "Operators scan. Rows are 32 px, type is 13–14 px, mono for identifiers. Density comes from alignment and hairlines, not from smaller text.",
    rule: "Row 32 px · table 28 px compact · body 14 px · labels 12 px mono. No text below 11 px.",
  },
  {
    n: "06",
    title: "Calm under load",
    body: "Forty sessions running should look as quiet as four. Motion, colour and sound are spent only on change, never on activity that is expected.",
    rule: "No animation loops longer than one cycle except the Working pulse; no colour on rows that are fine.",
  },
];

/**
 * The "everything loud" counter-example: every row in its own status colour. Each plate names
 * the ink its label needs for AA: the gradient ink on the bright plates, white only on volt.
 */
export const loud: { fill: string; ink: string }[] = [
  { fill: v("--color-destructive"), ink: v("--color-destructive-ink") },
  { fill: v("--color-attn"), ink: v("--color-on-gradient") },
  { fill: v("--color-brand-cyan"), ink: v("--color-on-gradient") },
  { fill: v("--color-success"), ink: v("--color-on-gradient") },
  { fill: v("--color-brand-volt"), ink: v("--color-fg-strong") },
  { fill: v("--color-brand-sky"), ink: v("--color-on-gradient") },
];

export interface Surf {
  token: string;
  hex: string;
  role: string;
  old?: string;
}
export const darkLadder: Surf[] = [
  {
    token: "--color-rail",
    hex: "#0a0a0a",
    role: "Rail, sidebar, the deepest layer",
    old: "#0A0A0A rail",
  },
  {
    token: "--color-canvas",
    hex: "#131313",
    role: "Canvas: the pane where work shows",
    old: "#131313 canvas",
  },
  {
    token: "--color-elevated",
    hex: "#1b1b1b",
    role: "Elevated: menus, popovers, cards",
    old: "#1C1C1C elevated",
  },
  {
    token: "--color-surface-2",
    hex: "#232323",
    role: "Hover and selected rows (proposed token)",
    old: "#232323 surface-2",
  },
];
export const lightLadder: Surf[] = [
  { token: "--color-rail (light)", hex: "#f6f6f6", role: "Rail, sidebar", old: "light rail" },
  { token: "--color-canvas (light)", hex: "#FFFFFF", role: "Canvas", old: "light canvas" },
  {
    token: "--color-elevated (light)",
    hex: "#ededed",
    role: "Elevated, wells, code",
    old: "light elevated",
  },
  {
    token: "--color-surface-2 (light)",
    hex: "#e5e5e5",
    role: "Hover and selected rows (proposed token)",
    old: "light surface-2",
  },
];
export const darkText: [string, string][] = [
  ["fg", "#ececec"],
  ["muted", "#989898"],
  ["faint", "#5e5e5e"],
  ["cyan", "#3DF2E0"],
  ["volt", "#6687FF"],
];
export const lightText: [string, string][] = [
  ["fg", "#0a0a0a"],
  ["muted", "#616161"],
  ["faint", "#a6a6a6"],
  ["cyan", "#0A6F86"],
  ["volt", "#2448E6"],
];
/** Hairlines and grade colours drawn inside each ladder specimen. */
export const ladderInk = {
  dark: { line: "#262626", lineStrong: "#383838" },
  light: {
    line: "#dcdcdc",
    lineStrong: "#cfcfcf",
    fail: "#C4302B",
    large: "#9A5B00",
    pass: "#0A6F86",
  },
};

export interface State {
  key: string;
  word: string;
  alt?: string;
  glyph: string;
  dark: string;
  light: string;
  tone: string;
  meaning: string;
}
export const states: State[] = [
  {
    key: "working",
    word: "Working",
    glyph: "●",
    dark: "#3DF2E0",
    light: "#0A6F86",
    tone: "cyan",
    meaning:
      "The agent is running a turn now. The only state that animates: the dot pulses once every 2.4 s.",
  },
  {
    key: "needs-you",
    word: "Needs you",
    alt: "Waiting for you",
    glyph: "◐",
    dark: "#F5B14C",
    light: "#9A5B00",
    tone: "warning",
    meaning:
      "A question, a tool approval or a sign-in. Sorts to the top; counts on the dock and tray.",
  },
  {
    key: "done",
    word: "Done",
    glyph: "✓",
    dark: "#34D399",
    light: "#0A7650",
    tone: "success",
    meaning: "The turn or Loop finished and its check passed. Shows elapsed time.",
  },
  {
    key: "idle",
    word: "Idle",
    glyph: "○",
    dark: "#989898",
    light: "#616161",
    tone: "muted",
    meaning: "Alive and quiet: waiting for the next prompt or trigger. No colour on purpose.",
  },
  {
    key: "failed",
    word: "Failed",
    glyph: "✕",
    dark: "#F87171",
    light: "#C4302B",
    tone: "danger",
    meaning: "The run ended with an error. The row keeps the last error line in muted text.",
  },
  {
    key: "stuck",
    word: "Stuck",
    glyph: "■",
    dark: "#F87171",
    light: "#C4302B",
    tone: "danger",
    meaning:
      "Still running, no progress past the stall threshold. Needs a look before it costs more.",
  },
  {
    key: "later",
    word: "Later",
    glyph: "◇",
    dark: "#cfcfcf",
    light: "#464646",
    tone: "neutral",
    meaning:
      "Parked by the operator with an optional return date. Out of the lists until it comes back.",
  },
];

/** Session-row chrome per theme: dark reads the product tokens, light carries the spec values. */
export const rowTheme = {
  dark: {
    fg: v("--color-fg"),
    muted: v("--color-muted-foreground"),
    sel: v("--color-surface-2"),
    bar: v("--color-brand-cyan"),
  },
  light: { fg: "#0a0a0a", muted: "#616161", sel: "#e5e5e5", bar: "#0A6F86" },
};

/** Status-table backgrounds the state colours are checked against. */
export const statusChecks: [name: string, hex: string, theme: "dark" | "light"][] = [
  ["ink-2", "#131313", "dark"],
  ["ink-4", "#232323", "dark"],
  ["white", "#FFFFFF", "light"],
  ["paper", "#f6f6f6", "light"],
];

export interface Session {
  name: string;
  agent: string;
  state: string;
  age: string;
  note?: string;
}
export const sessions: Session[] = [
  {
    name: "migrate-db",
    agent: "codex",
    state: "needs-you",
    age: "3m",
    note: "Approve: run migration on staging",
  },
  { name: "review-pr #412", agent: "claude-code", state: "working", age: "12m" },
  { name: "fix-flaky-auth", agent: "claude-code", state: "working", age: "4m" },
  {
    name: "release-notes",
    agent: "gemini",
    state: "stuck",
    age: "2h",
    note: "No output for 18 min",
  },
  { name: "triage-issues", agent: "cursor", state: "done", age: "41m" },
  { name: "nightly-deps", agent: "codex", state: "idle", age: "6h" },
  { name: "docs-sweep", agent: "claude-code", state: "later", age: "Mon 09:00" },
];

/** macOS system colours that appear in specimens (dock badge, menu bars). */
export const macos = {
  badge: "#FF453A",
  lightBar: "#ebebeb",
  lightBarText: "#1D1D1F",
  darkMenu: "#272727",
};

export const usage = [
  { agent: "claude-code", tokens: 4.8, cost: 38.4 },
  { agent: "codex", tokens: 3.1, cost: 21.7 },
  { agent: "cursor", tokens: 1.6, cost: 9.9 },
  { agent: "gemini", tokens: 0.9, cost: 4.1 },
];
/** sessions completed per day, last 14 days; last 3 projected; one unknown (daemon offline) */
export const daily: { d: string; v: number | null; projected?: boolean }[] = [
  { d: "24", v: 18 },
  { d: "25", v: 22 },
  { d: "26", v: 19 },
  { d: "27", v: 27 },
  { d: "28", v: null },
  { d: "29", v: 31 },
  { d: "30", v: 29 },
  { d: "31", v: 34 },
  { d: "01", v: 38 },
  { d: "02", v: 36 },
  { d: "03", v: 41 },
  { d: "04", v: 44, projected: true },
  { d: "05", v: 46, projected: true },
  { d: "06", v: 49, projected: true },
];
/** magnitude ramp, deep blue (low) to glint (high); ends are the brand palette tokens */
export const ramp = [
  v("--color-brand-deep"),
  "#1550FA",
  v("--color-brand-sky"),
  "#25B6EC",
  "#2DD9E1",
  "#7DF6EC",
  v("--color-brand-glint"),
];

export const cliHelp = `compozy 0.6.0 · the autonomous control plane for your agents

USAGE
  compozy <command> [flags]

CORE
  daemon        Start, stop and inspect the local runtime
  session       Create, prompt, resume and inspect sessions
  loop          Run work until a check passes, then stop
  task          Plan and review units of work
  automation    Jobs and triggers: cron, events, webhooks

FLAGS
  -o, --output  text | json | yaml        (default text)
      --color   auto | always | never     (default auto)
  -h, --help    Help for any command

Docs: https://compozy.com/docs`;

export const vocabulary: {
  term: string;
  icon: IconName;
  glyph?: string;
  visual: string;
  say: string;
  avoid: string;
}[] = [
  {
    term: "Session",
    icon: "session",
    glyph: "●",
    visual: "Board and desktop",
    say: "session, start a session",
    avoid: "chat, thread, conversation",
  },
  {
    term: "Loop",
    icon: "loop",
    visual: "Blue looping dot and counter",
    say: "Loop, run a Loop, the Loop stops",
    avoid: "agentic loop, autopilot",
  },
  {
    term: "Task",
    icon: "task",
    glyph: "✓",
    visual: "Done dot",
    say: "task, review a task",
    avoid: "ticket, job (for tasks)",
  },
  {
    term: "Automation",
    icon: "automation",
    visual: "Activity bars",
    say: "automation, job, trigger",
    avoid: "workflow, zap, cron (in UI copy)",
  },
  {
    term: "Trigger",
    icon: "trigger",
    visual: "Activity bars, first bar",
    say: "trigger, runs on",
    avoid: "hook (that is another thing)",
  },
  {
    term: "Agent",
    icon: "agent",
    visual: "Its face, with state mark and harness chip",
    say: "agent, the harness (Claude Code, Codex)",
    avoid: "bot, assistant, AI worker",
  },
  {
    term: "Harness",
    icon: "agent",
    visual: "Chip or mono label on the agent",
    say: "harness, runs on Codex",
    avoid: "wrapper, engine, model provider",
  },
  {
    term: "Control plane",
    icon: "desktop",
    visual: "Desktop",
    say: "control plane, the autonomous control plane for your agents",
    avoid: "dashboard, AI factory, platform",
  },
  {
    term: "Always-on agent",
    icon: "agent",
    glyph: "●",
    visual: "Agents list, faces",
    say: "Always-on, checks in on its own",
    avoid: "Stays on, 24/7",
  },
  {
    term: "Skill",
    icon: "skill",
    visual: "none",
    say: "skill, load a skill",
    avoid: "prompt pack, plugin",
  },
  {
    term: "Extension",
    icon: "extension",
    visual: "none",
    say: "extension, build an extension",
    avoid: "add-on, app (unless it adds an app)",
  },
  {
    term: "Terminal",
    icon: "terminal",
    visual: "Never the product image",
    say: "terminal",
    avoid: "console, shell window",
  },
  {
    term: "Vault",
    icon: "vault",
    visual: "none",
    say: "vault, secret",
    avoid: "keychain, password manager",
  },
  {
    term: "Tool approval",
    icon: "approval",
    glyph: "◐",
    visual: "Approval dock",
    say: "approve, allow once, always allow",
    avoid: "permission popup, consent",
  },
  {
    term: "Worktree",
    icon: "worktree",
    visual: "Run lanes",
    say: "worktree",
    avoid: "sandbox, branch (when it is a worktree)",
  },
  {
    term: "Memory",
    icon: "memory",
    visual: "Task stack",
    say: "memory",
    avoid: "brain, knowledge graph",
  },
  {
    term: "Desktop",
    icon: "desktop",
    visual: "Desktop",
    say: "desktop, window, rail",
    avoid: "workspace (taken), canvas",
  },
  {
    term: "Gateway",
    icon: "gateway",
    visual: "Orchestrate",
    say: "gateway, channel",
    avoid: "bridge, integration hub",
  },
  {
    term: "Network",
    icon: "network",
    glyph: "→",
    visual: "Orchestrate",
    say: "handoff, orchestration, network",
    avoid: "swarm, hive, multi-agent mesh",
  },
];
