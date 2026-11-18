// Copy for the Ads chapter. Components only map over these arrays.
// Compozy has no offer: no price, no plan, no trial deadline. Every ad is brand or proof.

export const modes = [
  {
    id: "brand",
    title: "Brand mode",
    goal: "Make an engineer recognise the problem and remember the name. One line, one Atmosphere visual, nothing to verify yet.",
    when: "Cold audiences: people who run Claude Code, Codex or Cursor in five terminals and have never seen Compozy.",
    shows: [
      "One hook of 6 words or fewer",
      "One Atmosphere visual (Swarm, Converge, Terrain, Horizon or Cone), cropped by the edge",
      'One CTA, usually "Install the beta"',
    ],
    never:
      "No numbers, no feature list, no product screen. If the piece needs proof to make sense, it belongs to proof mode.",
    example: "One desktop for every agent.",
    templates: ["ad-square-brand", "ad-x-card", "ad-dev-banner"],
  },
  {
    id: "proof",
    title: "Proof mode",
    goal: "Show the product doing its job in our own UI: the desktop, the agents, the board, the handoff, the approval. The reader can check every state against the docs.",
    when: "Warm audiences: site visitors, GitHub stargazers, people who saw a brand ad, engineering managers.",
    shows: [
      "One Live visual of our UI (desktop, agents, board, handoff or approval), never a terminal window",
      "A named state (needs you, working, done, approved)",
      'A CTA that goes one step deeper: the install page or "See what ships today"',
    ],
    never:
      "No invented screens, no features that do not exist, no counters that are not dated. Mock data uses the names from the product vocabulary.",
    example: "The agent asks. You decide.",
    templates: ["ad-square-proof", "ad-portrait", "ad-story", "ad-linkedin", "ad-landscape"],
  },
] as const;

export const nevers: [rule: string, why: string][] = [
  [
    'A price, a plan, a discount or "free trial"',
    "There is no paid tier. Self-hosting is free and the licence is MIT; a price in an ad invents a product that does not exist.",
  ],
  [
    '"Customers", "trusted by", or a logo wall',
    "The six companies are design partners implementing as an internal standard, unpaid. No names in ads, ever.",
  ],
  [
    'Revenue, growth rates or "companies use it"',
    "Zero revenue is the honest status. Adoption claims only for users the founder can name, and never in ad copy.",
  ],
  [
    "Production-grade, enterprise-ready, at scale",
    "It is beta. Ads that hide it get found out in the first install, and the first install is the whole funnel.",
  ],
  [
    'Fake urgency: "last chance", "today only", countdowns',
    "Nothing expires. Urgency without a real date reads as a growth trick to the exact audience being reached.",
  ],
  [
    "Named rivals or star comparisons",
    "The enemy is the DIY agent stack, never a product. Comparisons age badly and invite a reply thread.",
  ],
  [
    "Hype words, emoji and exclamation marks",
    "Banned list applies in full: seamless, effortless, AI-powered, next-gen, revolutionary. Calm reads as confident.",
  ],
  [
    "Generated people presented as users",
    "Generated images never stand for real team members, partners or users. Ads use Atmosphere visuals, product UI or code.",
  ],
];

export const steps = [
  {
    n: "01",
    title: "Hook",
    time: "0 – 1.5 s",
    size: "100%",
    body: "The reader's problem or the outcome, in 6 words or fewer. Largest type on the canvas, Instrument Sans 400, tracking −0.03em.",
  },
  {
    n: "02",
    title: "Mechanism",
    time: "1.5 – 4 s",
    size: "30 – 35%",
    body: "How it works, in one sentence with a noun the reader knows: session, harness, approval, board. Figtree, muted, 10 to 18 words.",
  },
  {
    n: "03",
    title: "Proof",
    time: "4 – 6 s",
    size: "27 – 30%",
    body: "Our UI as a Live visual: the desktop, the agents list, the board, the handoff picker or the approval dock. Brand mode replaces it with an Atmosphere visual; the visual is the mood of the brand.",
  },
  {
    n: "04",
    title: "CTA",
    time: "last read",
    size: "30%",
    body: "One volt button, sentence case, a verb first. The only volt element in the piece and the only thing that looks clickable.",
  },
];

export const hierarchy: [string, string][] = [
  [
    "Reading order",
    "Top left to bottom right. Hook first, CTA last. Nothing competes with the hook for the first 1.5 seconds.",
  ],
  ["One idea", "One ad, one claim. A second claim is a second ad, tested against the first."],
  [
    "Lockup",
    "Top left, 4cqi tall on squares (≈ 43 px at 1080). Never next to the CTA, never repeated.",
  ],
  [
    "Visual",
    "One per ad: Atmosphere for brand mode, Live (desktop, agents, board, handoff, approval) for proof mode. Live UI sits in its own panel or bleeds off one edge; text never runs over it without a scrim, and keeps 7:1 on ink.",
  ],
  [
    "Text on image",
    "Under 20% of the canvas area outside the hook. Feeds downrank text-heavy images and people skim past them.",
  ],
  [
    "Motion",
    "Optional. If the ad moves, the visual drifts slowly (one slow pass in 18 s) and the hook is on screen from frame one.",
  ],
];

export interface Model {
  id: string;
  mode: "brand" | "proof";
  where: string;
  guide: string[];
}

export const models: Model[] = [
  {
    id: "ad-square-brand",
    mode: "brand",
    where: "X, LinkedIn and Reddit feeds",
    guide: [
      "Hook up to 2 lines at 9.6cqi.",
      "Body is one sentence; drop it before you shrink the hook.",
      "Field visual stays low, cropped by the bottom edge, cropped by two edges.",
    ],
  },
  {
    id: "ad-square-proof",
    mode: "proof",
    where: "Retargeting on X and LinkedIn",
    guide: [
      "The approval dock fills the width of the panel: question, command, three buttons.",
      "The hook says who decides; the line below says what is allowed.",
      "CTA goes to the install page.",
    ],
  },
  {
    id: "ad-portrait",
    mode: "proof",
    where: "Mobile feeds at 4:5",
    guide: [
      "Hook top, handoff picker in the middle, mechanism and CTA below.",
      "The picker is the whole proof: name the two harnesses in the line under it.",
      "Use when the line needs a second sentence to land.",
    ],
  },
  {
    id: "ad-story",
    mode: "proof",
    where: "Full-screen vertical placements",
    guide: [
      "Nothing in the top 14% or bottom 20%: platform UI covers it.",
      "The always-on agents list shows faces and check-in notes, one row needing you.",
      "Status colours are product UI here, so green and amber are allowed.",
    ],
  },
  {
    id: "ad-landscape",
    mode: "proof",
    where: "Google display, Reddit link ads",
    guide: [
      "Text in the left 40%; the desktop bleeds off the right edge.",
      "Survives a crop to 1:1 from the left edge.",
      "Name the harnesses only in this order: Claude Code, Codex, Cursor.",
    ],
  },
  {
    id: "ad-x-card",
    mode: "brand",
    where: "X website cards",
    guide: [
      "X prints the domain over the bottom 14%: keep it empty.",
      "Hook at 7cqi, 2 lines at most.",
      "The post text carries the mechanism; the card carries the hook.",
    ],
  },
  {
    id: "ad-linkedin",
    mode: "proof",
    where: "LinkedIn sponsored content",
    guide: [
      "Speaks to the engineering manager: every agent session in one place.",
      "The board shows who holds the next step, never a metric.",
      'CTA "See what ships today", never "Book a demo".',
    ],
  },
  {
    id: "ad-dev-banner",
    mode: "brand",
    where: "GitHub sponsor slots and newsletters",
    guide: [
      "Shown at 300 to 600 px wide: hook at 4.6cqi, no body.",
      "Symbol on the left, never the full lockup; the agents list is texture, not copy.",
      "The link line is mono cyan and doubles as the CTA.",
    ],
  },
];

export const copyBank: {
  format: string;
  ids: string;
  headlines: string[];
  bodies: string[];
  ctas: string[];
}[] = [
  {
    format: "Square · X card · banner (brand)",
    ids: "ad-square-brand · ad-x-card · ad-dev-banner",
    headlines: [
      "One desktop for every agent.",
      "Five terminals. One desktop.",
      "Stop managing sessions.",
      "Your agents, one control plane.",
    ],
    bodies: [
      "The autonomous control plane for the agents your team already uses.",
      "The bottleneck stopped being the model. It became managing the sessions.",
      "See and command Claude Code, Codex and Cursor from one place.",
    ],
    ctas: ["Install the beta", "Read the docs", "compozy.com →"],
  },
  {
    format: "Square · portrait · story (proof)",
    ids: "ad-square-proof · ad-portrait · ad-story",
    headlines: [
      "The agent asks. You decide.",
      "Continue with another agent.",
      "Agents that check in.",
      "See what needs you.",
    ],
    bodies: [
      "Allow once, always allow or deny, right in the session, across every harness.",
      "Hand a session from Claude Code to Codex with its context, files and worktree.",
      "Always-on agents work between triggers and call you only when they need a decision.",
    ],
    ctas: ["Install the beta", "See what ships today"],
  },
  {
    format: "LinkedIn",
    ids: "ad-linkedin",
    headlines: [
      "Every agent session, one board.",
      "One control plane, every engineer.",
      "Approvals your team can audit.",
    ],
    bodies: [
      "See who holds the next step across Claude Code, Codex and Cursor. Approve, hand off or step in from one place.",
      "Child agents never get wider permissions than the parent that started them.",
      "Open source, MIT, local-first. Beta today, and honest about what ships.",
    ],
    ctas: ["See what ships today", "Read the docs"],
  },
  {
    format: "Landscape",
    ids: "ad-landscape",
    headlines: [
      "One desktop for every agent.",
      "Orchestrate, do not babysit.",
      "Built to be built on.",
    ],
    bodies: [
      "See and command Claude Code, Codex and Cursor from one control plane. Always-on agents call you only when needed.",
      "One orchestrator session hands work to child sessions in other harnesses and worktrees.",
      "Extensions, hooks and skills on the same contracts people use.",
    ],
    ctas: ["Install the beta", "Build an extension"],
  },
];

export const rewrites: [dont: string, doIt: string][] = [
  ["The AI-powered platform trusted by leading companies.", "One desktop for every agent."],
  ["Supercharge your agents! Try it free today.", "Install the beta. MIT, self-hosted."],
  ["Enterprise-grade orchestration at scale.", "Every agent session, one board."],
];

export const specRows: {
  channel: string;
  sizes: string;
  text: string;
  limits: string;
  note: string;
}[] = [
  {
    channel: "X",
    sizes: "800×418 card · 1080×1080 · 1080×1350",
    text: "Post 280 · card headline 70",
    limits: "PNG/JPG ≤ 5 MB",
    note: "Domain overlays the card bottom left. Post text carries the mechanism.",
  },
  {
    channel: "LinkedIn",
    sizes: "1200×627 · 1080×1080 · 1080×1350",
    text: "Intro 600 (≈ 150 shown) · headline 200 (≈ 70 shown)",
    limits: "PNG/JPG ≤ 5 MB",
    note: 'Front-load the first 150 characters; the rest hides behind "see more".',
  },
  {
    channel: "Reddit",
    sizes: "1200×628 · 1080×1080 · 1080×1350",
    text: "Headline 300 (≈ 100 read)",
    limits: "PNG/JPG ≤ 3 MB",
    note: "Comments are on by default. Answer in the founder's voice, plainly.",
  },
  {
    channel: "Google · display",
    sizes: "1200×628 · 1200×1200 · logo 1200×1200",
    text: "Short headline 30 · long 90 · description 90 · name 25",
    limits: "≤ 5,120 KB",
    note: "Responsive display recombines assets; every headline must stand alone.",
  },
  {
    channel: "Google · search",
    sizes: "text only",
    text: "Headline 30 ×15 · description 90 ×4 · path 15 ×2",
    limits: "—",
    note: 'Pin the brand headline to position 1. No superlatives, no "free".',
  },
  {
    channel: "Meta stories",
    sizes: "1080×1920",
    text: "Primary 125 · headline 40",
    limits: "PNG/JPG ≤ 30 MB",
    note: "Top 14% and bottom 20% are covered by UI.",
  },
  {
    channel: "Dev newsletters",
    sizes: "600×200 banner where offered",
    text: "Title ≈ 60 · body 50 – 100 words",
    limits: "per publisher",
    note: "Text-first. Write it like a changelog entry, not like an ad.",
  },
];

export const utm: [param: string, rule: string, example: string][] = [
  ["utm_source", "Platform, lowercase, one word", "x · linkedin · reddit · google · tldr"],
  ["utm_medium", "Buying type", "paid-social · paid-search · display · newsletter · sponsor"],
  ["utm_campaign", "yyyy-mm_goal_theme, kebab-case", "2026-11_install_desktop"],
  ["utm_content", "Template id and variant letter", "ad-square-proof_a"],
  ["utm_term", "Search keyword only", "agent-control-plane"],
];

export const utmExample =
  "https://compozy.com/docs/install?utm_source=x&utm_medium=paid-social&utm_campaign=2026-11_install_desktop&utm_content=ad-square-proof_a";

export const checklist: [group: string, items: string[]][] = [
  [
    "Claims",
    [
      "No price, plan, discount or trial anywhere, including the landing page above the fold.",
      "Every number is dated (Nov 2026) and listed in the proof points.",
      "Design partners are never called customers, and no company is named.",
      '"Beta" is visible on the ad or the first screen it links to.',
    ],
  ],
  [
    "Voice",
    [
      "Hook has 6 words or fewer and one idea.",
      'No banned words, no emoji, no exclamation marks, no "we".',
      "CTA starts with a verb and matches the page it opens.",
      "Agent CLIs named in the order Claude Code, Codex, Cursor.",
    ],
  ],
  [
    "Visual",
    [
      "Exported from /kit at native size, never from a screenshot.",
      "Text inside the margin box; nothing in hatched unsafe bands.",
      "One volt element: the CTA. Blue and cyan only inside the visual.",
      "Hook and body keep 7:1 contrast on their scrim.",
    ],
  ],
  [
    "Proof",
    [
      "Every screen shown exists in the current desktop, redrawn by a Live visual.",
      "Mock sessions use product vocabulary: Needs you, Working, Waiting, In review, Done.",
      "No terminal window as the image; one small install line is allowed as a detail.",
      "The page behind the CTA shows the same view.",
    ],
  ],
  [
    "Tracking",
    [
      "UTM follows the convention; utm_content is the template id.",
      "Link opens on compozy.com or the docs, never a redirect shortener.",
      "Variant letters logged before launch, with the one hypothesis being tested.",
      "Spend cap and end date set on the platform.",
    ],
  ],
];
