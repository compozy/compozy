// Copy for the Foundations chapter. Components only map over these arrays.

export const essence: [label: string, text: string][] = [
  [
    "Purpose",
    "Let people run many agents at once without becoming their switchboard. The agents work on their own; you step in when they need you.",
  ],
  ["Positioning", "The autonomous control plane for your agents."],
  [
    "Category",
    "Control plane for multiple harnesses. A harness runs the model (Claude Code, Codex, Gemini); the control plane is where you see and command all of them.",
  ],
  [
    "Promise",
    "Every agent, every harness, one desktop: always-on agents that check in on their own, sessions that hand work to each other, and you only where a decision is needed.",
  ],
  [
    "The enemy",
    "Five terminals and the prompt in the wrong window. Managing sessions by hand became the bottleneck.",
  ],
  [
    "Proof style",
    "Our UI doing the work: a face that needs you, a board by who has the next step, a handoff to another harness. Never an adjective.",
  ],
];

export const values: [title: string, text: string][] = [
  [
    "Every harness, one place",
    "Claude Code, Codex, Gemini and the rest run side by side, with your own logins. Nobody has to pick one vendor.",
  ],
  [
    "Calm under load",
    "Twelve agents running reads like one. The interface stays quiet so the exception is obvious.",
  ],
  [
    "Visible state",
    "Every run has a state you can see: working, waiting for you, stuck, done. No hidden bills.",
  ],
  [
    "Bounded autonomy",
    "Agents act on their own inside limits a person set. Children never get wider permissions than the parent.",
  ],
  [
    "Honest status",
    "Beta is beta. We say what ships today and name the limits before anyone finds them.",
  ],
];

export interface Persona {
  name: string;
  role: string;
  pain: string;
  wants: string;
  trigger: string;
  cta: string;
}
export const personas: Persona[] = [
  {
    name: "The operator",
    role: "Developer already running Claude Code, Codex or Cursor every day",
    pain: "Agents stop when the tab closes. Loops, cron and approvals are glued together by hand.",
    wants: "A first loop that works, visible session state, commands one step deeper.",
    trigger: "The third time a long run died overnight.",
    cta: "Install the beta",
  },
  {
    name: "The builder",
    role: "Agent, runtime and extension developer",
    pain: "Every harness exposes a different contract; nothing composes.",
    wants: "Stable APIs, SDKs, the tool registry, hooks and capabilities.",
    trigger: "Needing one integration to work across every agent CLI.",
    cta: "Build an extension",
  },
  {
    name: "The evaluator",
    role: "Engineer comparing CLIs, harnesses, MCP, A2A and workflow engines",
    pain: "Everything claims to orchestrate agents. Little says what it actually ships.",
    wants: "Sharp positioning, named constraints, honest maturity, sourced comparison.",
    trigger: "A team decision about the agent process.",
    cta: "See what ships today",
  },
  {
    name: "The engineering lead",
    role: "Owner of how a whole org works with agents",
    pain: "Same tools, one team ships 3x, another 10x. Nothing repeats across the org.",
    wants: "Standard process, approvals, audit and shared state.",
    trigger: "Rolling agents out past the first enthusiasts.",
    cta: "Talk to the founder",
  },
];

/**
 * Swatch content: name, documented v1.2.0 hex, role. `token` is the @compozy/ui token that
 * carries the colour (ADR-002) and `fill`/`ink` are CSS values built from tokens, so the
 * specimen renders what the product ships.
 */
export interface Swatch {
  name: string;
  hex: string;
  token: string;
  role: string;
  fill: string;
  ink: string;
}

const v = (token: string) => `var(${token})`;
const ON_INK = v("--color-fg");
const ON_LIGHT = v("--color-rail");
/** secondary text on paper, derived from the ink and light tokens */
const INK_MUTED = "color-mix(in oklab, var(--color-rail) 62%, var(--color-fg))";

export const colors: { core: Swatch[]; gradient: Swatch[]; neutrals: Swatch[] } = {
  core: [
    {
      name: "Ink",
      hex: "#0a0a0a",
      token: "--color-rail",
      role: "The ground. Every page, screen and piece starts here.",
      fill: v("--color-rail"),
      ink: ON_INK,
    },
    {
      name: "Volt",
      hex: "#2F5BFF",
      token: "--color-brand-volt",
      role: "Action on paper; on ink the action colour is signal blue (Sky). Primary buttons, links, focus, selected state.",
      fill: v("--color-brand-volt"),
      ink: v("--color-fg-strong"),
    },
    {
      name: "Signal",
      hex: "#3DF2E0",
      token: "--color-brand-cyan",
      role: "The one accent. Live state, mono labels, the OS chip.",
      fill: v("--color-brand-cyan"),
      ink: ON_LIGHT,
    },
    {
      name: "Paper",
      hex: "#f6f6f6",
      token: "--color-fg",
      role: "The light ground for docs, print and long reading.",
      fill: v("--color-fg"),
      ink: ON_LIGHT,
    },
  ],
  gradient: [
    {
      name: "Sky",
      hex: "#1F87FC",
      token: "--color-brand-sky",
      role: "Gradient start. The action colour on ink (--color-brand in dark) and the info state.",
      fill: v("--color-brand-sky"),
      ink: v("--color-on-gradient"),
    },
    {
      name: "Teal",
      hex: "#1DF0D7",
      token: "--color-brand-teal",
      role: "Gradient only. Never a flat fill.",
      fill: v("--color-brand-teal"),
      ink: ON_LIGHT,
    },
    {
      name: "Glint",
      hex: "#B5FAF6",
      token: "--color-brand-glint",
      role: "Gradient end. The light on the fold.",
      fill: v("--color-brand-glint"),
      ink: ON_LIGHT,
    },
  ],
  neutrals: [
    {
      name: "Ink 2",
      hex: "#131313",
      token: "--color-canvas",
      role: "Raised panels, cards",
      fill: v("--color-canvas"),
      ink: ON_INK,
    },
    {
      name: "Ink 3",
      hex: "#1b1b1b",
      token: "--color-elevated",
      role: "Overlays, selected rows",
      fill: v("--color-elevated"),
      ink: ON_INK,
    },
    {
      name: "Line",
      hex: "#262626",
      token: "--color-line",
      role: "Hairlines on ink",
      fill: v("--color-line"),
      ink: ON_INK,
    },
    {
      name: "Line strong",
      hex: "#383838",
      token: "--color-line-strong",
      role: "Chip borders, focus rings at rest",
      fill: v("--color-line-strong"),
      ink: ON_INK,
    },
    {
      name: "Faint",
      hex: "#5e5e5e",
      token: "--color-faint",
      role: "Decoration only, never text",
      fill: v("--color-faint"),
      ink: ON_INK,
    },
    {
      name: "Muted",
      hex: "#989898",
      token: "--color-muted-foreground",
      role: "Secondary text on ink",
      fill: v("--color-muted-foreground"),
      ink: ON_LIGHT,
    },
    {
      name: "Fg",
      hex: "#ececec",
      token: "--color-fg",
      role: "Primary text on ink",
      fill: v("--color-fg"),
      ink: ON_LIGHT,
    },
    {
      name: "Ink muted",
      hex: "#616161",
      token: "—",
      role: "Secondary text on paper",
      fill: INK_MUTED,
      ink: v("--color-fg-strong"),
    },
  ],
};

/** text pairs that must pass AA; ratios are computed from the documented hex in the component */
export const contrastPairs: {
  fg: string;
  bg: string;
  use: string;
  fgFill: string;
  bgFill: string;
}[] = [
  {
    fg: "#ececec",
    bg: "#0a0a0a",
    use: "Body text on ink",
    fgFill: v("--color-fg"),
    bgFill: v("--color-rail"),
  },
  {
    fg: "#989898",
    bg: "#0a0a0a",
    use: "Secondary text on ink",
    fgFill: v("--color-muted-foreground"),
    bgFill: v("--color-rail"),
  },
  {
    fg: "#3DF2E0",
    bg: "#0a0a0a",
    use: "Signal labels on ink",
    fgFill: v("--color-brand-cyan"),
    bgFill: v("--color-rail"),
  },
  {
    fg: "#FFFFFF",
    bg: "#2F5BFF",
    use: "Button label on volt (paper)",
    fgFill: v("--color-fg-strong"),
    bgFill: v("--color-brand-volt"),
  },
  {
    fg: "#0a0a0a",
    bg: "#f6f6f6",
    use: "Body text on paper",
    fgFill: v("--color-rail"),
    bgFill: v("--color-fg"),
  },
  {
    fg: "#616161",
    bg: "#f6f6f6",
    use: "Secondary text on paper",
    fgFill: INK_MUTED,
    bgFill: v("--color-fg"),
  },
  {
    fg: "#0A6F86",
    bg: "#f6f6f6",
    use: "OS chip on paper",
    fgFill: "color-mix(in oklab, var(--color-brand-cyan) 45%, var(--color-rail))",
    bgFill: v("--color-fg"),
  },
  {
    fg: "#0a0a0a",
    bg: "#1F87FC",
    use: "Button label on signal blue (ink)",
    fgFill: v("--color-brand-ink"),
    bgFill: v("--color-brand"),
  },
  {
    fg: "#4A9DFF",
    bg: "#0a0a0a",
    use: "Brand text and links on ink",
    fgFill: v("--color-brand-strong"),
    bgFill: v("--color-rail"),
  },
  {
    fg: "#0a0a0a",
    bg: "#1F87FC",
    use: "Text on the gradient, against its darkest stop (never white)",
    fgFill: v("--color-on-gradient"),
    bgFill: v("--gradient-brand"),
  },
  {
    fg: "#2F5BFF",
    bg: "#0a0a0a",
    use: "Volt text on ink (large only)",
    fgFill: v("--color-brand-volt"),
    bgFill: v("--color-rail"),
  },
];

export const proportion: [name: string, fill: string, pct: number][] = [
  ["Ink", v("--color-rail"), 62],
  ["Ink 2", v("--color-canvas"), 14],
  ["Fg", v("--color-fg"), 10],
  ["Gradient", v("--gradient-brand"), 8],
  ["Volt", v("--color-brand-volt"), 4],
  ["Signal", v("--color-brand-cyan"), 2],
];

export const typeScale: [name: string, cls: string, spec: string, sample: string][] = [
  [
    "Display",
    "font-display text-[72px] leading-[1] tracking-[-0.03em] font-normal",
    "Instrument Sans 400 · 48–112 px · −3%",
    "Agents that keep running.",
  ],
  [
    "Title",
    "font-display text-[44px] leading-[1.05] tracking-[-0.025em] font-normal",
    "Instrument Sans 400 · 34–60 px · −2.5%",
    "The process, already built.",
  ],
  [
    "Heading",
    "font-display text-[26px] leading-[1.15] tracking-[-0.015em] font-medium",
    "Instrument Sans 500 · 22–28 px · −1.5%",
    "Loops with a definition of done",
  ],
  [
    "Lead",
    "font-sans text-[21px] leading-[1.5]",
    "Figtree 400 · 19–22 px",
    "One complete environment to create, automate and supervise agent work.",
  ],
  [
    "Body",
    "font-sans text-[17px] leading-[1.6]",
    "Figtree 400 · 15–17 px",
    "Agents keep working after the tab closes, with saved history, resumable state and the same view from CLI, API and web.",
  ],
  [
    "UI",
    "font-sans text-[14px] leading-[1.45] font-medium",
    "Figtree 500 · 13–14 px",
    "Waiting for you · Move to Later",
  ],
  [
    "Mono label",
    "font-mono text-[12px] uppercase tracking-[0.12em]",
    "Geist Mono 500 · 11–13 px · +12%",
    "● 12 running · always-on",
  ],
  [
    "Code",
    "font-mono text-[14px] leading-[1.7]",
    "Geist Mono 400 · 13–15 px",
    "$ compozy loop start review-pr",
  ],
];

export * from "./voice-data";
