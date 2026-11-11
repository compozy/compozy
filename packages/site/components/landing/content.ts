/** Approved landing copy (`.compozy/tasks/landing-redesign/landing-copy.md`), kept as data so the sections stay layout. */
import {
  Bookmark,
  Compass,
  Folder,
  Gauge,
  Globe,
  HeartPulse,
  KeyRound,
  Layers,
  ListChecks,
  Monitor,
  MessageSquare,
  Plug,
  RefreshCw,
  Sparkles,
  Terminal,
  Users,
  Webhook,
  type LucideIcon,
} from "@compozy/ui/icons";

import { ASSET } from "@/components/landing/assets";
import {
  HOMEBREW_INSTALL_COMMAND,
  NPM_INSTALL_COMMAND,
  VERIFIED_INSTALLER_COMMAND,
  goInstallCommand,
} from "@/lib/release/install-commands";

export const INSTALL_COMMAND = VERIFIED_INSTALLER_COMMAND;

/** COPY.md §2 Hero Lock: category label, headline and subhead ship together, verbatim. */
export const HERO_CATEGORY = "An operating system for AI agents";
export const HERO_HEADLINE = "The autonomous control plane for your agents.";
export const HERO_SUBHEAD =
  "See and steer every Claude Code, Codex, and Cursor session. Always-on agents check in on their own, and you step in only when one needs you.";

/** Where the landing's calls to action lead. */
export const HREF = {
  download: "/download",
  docs: "/docs",
  providers: "/docs/agents/providers",
  alwaysOn: "/docs/guides/watch-new-github-issues",
  loops: "/docs/loops",
  marketplace: "/marketplace",
  buildExtension: "/docs/extensions/develop",
} as const;

export const QUOTES = [
  ["This tool is spectacular.", "Fabio Vedovelli"],
  ["At work, employee of the week is going to be Compozy.", "João"],
  ["I left it running and went to a family barbecue.", "Jefferson Veloso"],
] as const;

export const FEATURES: ReadonlyArray<{
  tab: string;
  icon: LucideIcon;
  title: string;
  body: string;
  shot: string;
  alt: string;
}> = [
  {
    tab: "Sessions",
    icon: MessageSquare,
    title: "Agent work that outlives the terminal",
    body: "Fork a session, continue it with another agent, rewind to a checkpoint, or steer it mid-turn.",
    shot: ASSET.session,
    alt: "A running session with Fork from here on each message and Queue, Steer and Interrupt in the composer",
  },
  {
    tab: "Files",
    icon: Folder,
    title: "See what changed without leaving the session",
    body: "Browse and search the project, and review a session's diff against its base.",
    shot: ASSET.files,
    alt: "A session with its Changes panel open on a diff against the base",
  },
  {
    tab: "Terminal",
    icon: Terminal,
    title: "Shells people and agents share",
    body: "Runtime-owned terminals you can open, watch, and hand to an agent.",
    shot: ASSET.terminal,
    alt: "The integrated terminal after a test run in a project",
  },
  {
    tab: "Desktops",
    icon: Monitor,
    title: "Every window managed",
    body: "Persistent desktops, tiled and floating windows, a command palette, and a global hotkey.",
    shot: ASSET.desktops,
    alt: "A desktop with sessions, a terminal and tasks tiled side by side",
  },
  {
    tab: "Workspaces",
    icon: Layers,
    title: "One project, registered once",
    body: "See all projects at once, work without a project, and give parallel agents their own worktree.",
    shot: ASSET.workspaces,
    alt: "Sessions grouped by project with the project picker open",
  },
  {
    tab: "Tasks",
    icon: ListChecks,
    title: "Work that stays claimed until it closes",
    body: "Durable tasks a session claims, heartbeats, and closes with evidence.",
    shot: ASSET.tasks,
    alt: "The Tasks board with pending, blocked and done work",
  },
  {
    tab: "Profiles",
    icon: Users,
    title: "Separate work on one install",
    body: "Each session and automation files under a named profile. Profiles organize work; they are not a security boundary.",
    shot: ASSET.profiles,
    alt: "The profile switcher with several named profiles",
  },
  {
    tab: "Gateway",
    icon: Globe,
    title: "Local until you turn remote on",
    body: "The daemon starts reachable only on this machine. Remote access is off by default.",
    shot: ASSET.gateway,
    alt: "Remote access settings, off by default",
  },
];

export const DETAILS: ReadonlyArray<readonly [LucideIcon, string, string]> = [
  [Bookmark, "Later", "Park a session with a reason and a date."],
  [Folder, "Folders and bulk actions", "Group sessions, then move or archive them together."],
  [Gauge, "Provider usage (experimental)", "See subscription limits and when they reset."],
  [RefreshCw, "One-click update", "Update the app and the runtime from the menu bar."],
  [KeyRound, "Vault", "Keep secrets out of your config files."],
  [Webhook, "Hooks", "Run your own code on daemon events."],
  [Sparkles, "Skills", "Give agents reusable instructions from any source."],
  [Plug, "MCP sign-in", "Sign in to MCP servers and attach them to agents."],
  [Compass, "Model catalog", "Curate the models each provider offers."],
  [HeartPulse, "Health", "Spot stuck or unhealthy sessions."],
];

export const COMPARE_COLUMNS = ["Synara", "T3 Code", "Hermes", "OpenClaw", "DIY stack"] as const;

export type CompareColumn = (typeof COMPARE_COLUMNS)[number];
export type CompareVerdict = "yes" | "partial" | "no";
type CompareRowVerdicts = readonly [
  CompareVerdict,
  CompareVerdict,
  CompareVerdict,
  CompareVerdict,
  CompareVerdict,
  CompareVerdict,
];

/** When each product's public documentation was last checked for the table below. */
export const COMPARE_CHECKED_ON = "November 10, 2026";

/** The public documentation each product column was checked against. */
export const COMPARE_SOURCES: Record<Exclude<CompareColumn, "DIY stack">, string> = {
  Synara: "https://www.trysynara.com/docs",
  "T3 Code": "https://github.com/pingdotgg/t3code",
  Hermes: "https://hermes-agent.nousresearch.com/docs",
  OpenClaw: "https://docs.openclaw.ai",
};

/**
 * One verdict per row of `COMPARE_ROWS`, in order, from each product's own documentation
 * (checked on `COMPARE_CHECKED_ON`). "partial" means the docs cover part of the row: for example
 * approvals without an audit trail, or schedules without event triggers. Rows a product's docs do
 * not mention read "no". The DIY stack column is the assembled alternative (agent CLIs plus cron,
 * scripts and terminals), so it is a judgment rather than a documented product.
 */
export const COMPARE_VERDICTS: Record<CompareColumn, CompareRowVerdicts> = {
  // Agent CLIs: yes. Heartbeat runs and goals, no proactive agent: partial. "Awaiting you" board:
  // yes. Schedules and loops, no event triggers: partial. Approvals, audit only for external MCP:
  // partial. Local, MIT: yes.
  Synara: ["yes", "partial", "yes", "partial", "partial", "yes"],
  // Agent CLIs: yes. Background service and push alerts, no proactive agent: partial. Sidebar by
  // working state, not a board: partial. Goals, schedules, webhooks: yes. Permission modes, no
  // audit log: partial. Local, MIT: yes.
  "T3 Code": ["yes", "partial", "partial", "yes", "partial", "yes"],
  // Its own agent; Codex only as an opt-in runtime: partial. Heartbeat and goals: yes. Task kanban
  // by status: partial. Loops, cron, GitHub webhooks: yes. Approvals, session history but no
  // audit: partial. Local, MIT: yes.
  Hermes: ["partial", "yes", "partial", "yes", "partial", "yes"],
  // ACP coding harnesses: yes. Proactive check job: yes. No board by next owner: no. Scheduler and
  // webhooks, no goal loops: partial. Exec approvals, no audit trail: partial. Local, MIT: yes.
  OpenClaw: ["yes", "yes", "no", "partial", "partial", "yes"],
  // The CLIs run as they are; nothing checks in, nothing groups sessions; cron and scripts cover
  // part of loops; each CLI prompts on its own with no shared record; licences vary by tool.
  "DIY stack": ["yes", "no", "no", "partial", "partial", "partial"],
};

export const COMPARE_ROWS = [
  "Runs the agents you already use",
  "Always-on agents that check in on their own",
  "One board by who owns the next step",
  "Loops, schedules, and triggers",
  "Approvals and audit",
  "Local-first and MIT",
];

/** Terminal install methods; the Go line pins the current release tag. */
export function installMethods(tag: string) {
  return [
    { id: "Installer", command: VERIFIED_INSTALLER_COMMAND },
    { id: "Homebrew", command: HOMEBREW_INSTALL_COMMAND },
    { id: "npm", command: NPM_INSTALL_COMMAND },
    { id: "Go", command: goInstallCommand(tag) },
  ];
}

/**
 * First-run steps after any install method. Homebrew, npm and Go ship only the binary, so the
 * provider setup stays an explicit step ahead of the daemon.
 */
export const INSTALL_STEPS: ReadonlyArray<{ title: string; command: string; note?: string }> = [
  {
    title: "Pick a provider",
    command: "compozy install",
    note: "The verified installer opens this for you.",
  },
  { title: "Start the daemon", command: "compozy daemon start" },
  {
    title: "Open a session",
    command: "compozy session new --agent general --name first-run",
    note: "from the repository you want to work on",
  },
];
