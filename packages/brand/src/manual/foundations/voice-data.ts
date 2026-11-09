// Voice and messaging data for the Foundations chapter (split from data.ts to keep it under 500 lines).
export const voiceAttrs: { name: string; is: string; isnt: string; do: string; dont: string }[] = [
  {
    name: "Calm",
    is: "Steady, specific, unhurried.",
    isnt: "Hype, urgency, exclamation marks.",
    do: "An agent that stopped making progress becomes a state you can see, not a bill you discover later.",
    dont: "Never lose an agent run again!!",
  },
  {
    name: "Plain",
    is: "Everyday verbs first, the runtime noun one step deeper.",
    isnt: "Jargon as a costume.",
    do: "Agents keep working after the tab closes.",
    dont: "A runtime environment that wraps AI agents with the infrastructure they need.",
  },
  {
    name: "Product-led",
    is: "The product is the subject. Mechanism, command, artifact.",
    isnt: '"We" and "our" in marketing copy.',
    do: "Compozy keeps each agent in a durable session with saved history and resumable state.",
    dont: "We built an amazing platform for your agents.",
  },
  {
    name: "Honest",
    is: "Names the limits and the maturity.",
    isnt: "Enterprise-grade claims for a beta.",
    do: "Honest status: it's beta, MIT-licensed, self-hosted, no telemetry, no billing.",
    dont: "The only true OS for AI agents, trusted by enterprises.",
  },
];

export const banned: [word: string, why: string][] = [
  ["wrapper, wraps", "Compozy is the system, not a layer around someone else’s."],
  [
    "AI-powered, revolutionary, seamless, cutting-edge, next-gen, game-changing",
    "Adjectives instead of proof.",
  ],
  ["simple, easy (without a metric)", 'Say "already built", "nothing to assemble", "built in".'],
  [
    "AI factory",
    "Occupied by datacenter vendors. Factory describes what runs on Compozy, never Compozy.",
  ],
  [
    "customers (for design partners)",
    "Six companies are implementing as design partners. Name them only when asked.",
  ],
  ["Stays on", "Retired. The label is Always-on, against On-demand."],
  ["chat (for a session)", "Sessions are durable runs, not conversations."],
  ["emoji, !, fake urgency", "Calm, not cute."],
];

export const glossary: [term: string, meaning: string][] = [
  [
    "Compozy",
    'The product and the company. "CompozyOS" only where the OS context matters; `compozy` is the CLI.',
  ],
  ["Session", "A durable agent run with saved history and resumable state."],
  [
    "Loop",
    "Repeatable work: goal → act → verify → stop, with a definition of done and named end states.",
  ],
  ["Automation", "Agent work started by a schedule, an event or a webhook."],
  ["Always-on", "An agent that keeps its session warm between messages. Not 24/7, not cron."],
  ["Skill", "A bundled procedure an agent can load when the task calls for it."],
  ["Extension", "Code that adds tools, hooks or surfaces through the same contracts people use."],
  ["Harness", "An agent CLI Compozy drives: Claude Code, Codex, Cursor and 23 more over ACP."],
  [
    "Design partner",
    "A company implementing Compozy as its internal standard, unpaid, during the beta.",
  ],
  [
    "Control plane",
    "Where you see and command every harness at once: start, supervise, route, approve and stop agent work. What Compozy is.",
  ],
  [
    "Harness",
    "The program that runs the model: Claude Code, Codex, Gemini CLI, OpenCode, Cursor. Compozy runs the real one with your own login or key.",
  ],
  [
    "Always-on agent",
    "An agent with a name, a face and a main session that checks in on its own, stays quiet when there is nothing to say, and calls you when something needs attention. Not 24/7, not cron.",
  ],
];

export const messages: { level: string; text: string; use: string }[] = [
  {
    level: "Positioning",
    text: "The autonomous control plane for your agents.",
    use: "Hero, social bios, deck cover, OG default.",
  },
  {
    level: "Explainer",
    text: "Run Claude Code, Codex, Gemini and the rest from one desktop. They work on their own and call you when they need you.",
    use: "Subhead and the line people repeat.",
  },
  {
    level: "Pain",
    text: "The bottleneck stopped being the model. It became managing the sessions.",
    use: "Openings, talks, ads. Follow it with the five-terminals picture.",
  },
  {
    level: "Answer",
    text: "Compozy is where you see and command all of them.",
    use: "Close of the pain line.",
  },
  {
    level: "Proof",
    text: "Always-on agents, orchestration across harnesses, a board by who has the next step, approvals in one click.",
    use: "Feature row, under the hero.",
  },
  {
    level: "Team line",
    text: "Same tools, one team ships 3x, another 10x. The difference is a process the agents can run.",
    use: "Investor and team pages. Attributed to teaching 3,000+ developers.",
  },
  {
    level: "Factory bridge",
    text: "The OS your agent factory runs on.",
    use: "Talks and long-form only.",
  },
  { level: "Sign-off", text: "Keep every agent running.", use: "Footers, end cards, stickers." },
];

export const ctas: [label: string, where: string][] = [
  ["Install the beta", "Primary, everywhere"],
  ["Start the daemon", "Docs, after install"],
  ["Open the runtime docs", "Secondary on landing"],
  ["Build an extension", "Builder pages"],
  ["See what ships today", "Evaluator pages"],
];
