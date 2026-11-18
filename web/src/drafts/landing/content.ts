/** Approved copy (`.compozy/tasks/landing-redesign/wireframe.html`), kept as data so the sections stay layout. */
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

import filesShot from "./assets/s-files.jpg";
import gatewayShot from "./assets/s-gateway.jpg";
import workspacesShot from "./assets/s-workspaces.jpg";
import desktopsShot from "./assets/s-desktops.jpg";
import profilesShot from "./assets/s-profiles.jpg";
import sessionShot from "./assets/s-session.jpg";
import tasksShot from "./assets/s-tasks.jpg";
import terminalShot from "./assets/s-terminal.jpg";

export const INSTALL_COMMAND = "curl -fsSL https://compozy.com/install.sh | sh";

/** Prototype snapshot of live values; production reads them from GitHub and `BUILTIN_PROVIDER_COUNT`. */
export const LIVE = { stars: 2860, releases: 79, providers: 15 };

export const NAV = ["Docs", "Marketplace", "Blog", "Changelog", "Download"];

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
    shot: sessionShot,
    alt: "A running session with Fork from here on each message and Queue, Steer and Interrupt in the composer",
  },
  {
    tab: "Files",
    icon: Folder,
    title: "See what changed without leaving the session",
    body: "Browse and search the project, and review a session's diff against its base.",
    shot: filesShot,
    alt: "A session with its Changes panel open on a diff against the base",
  },
  {
    tab: "Terminal",
    icon: Terminal,
    title: "Shells people and agents share",
    body: "Runtime-owned terminals you can open, watch, and hand to an agent.",
    shot: terminalShot,
    alt: "The integrated terminal after a test run in a project",
  },
  {
    tab: "Desktops",
    icon: Monitor,
    title: "Every window managed",
    body: "Persistent desktops, tiled and floating windows, a command palette, and a global hotkey.",
    shot: desktopsShot,
    alt: "A desktop with sessions, a terminal and tasks tiled side by side",
  },
  {
    tab: "Workspaces",
    icon: Layers,
    title: "One project, registered once",
    body: "See all projects at once, work without a project, and give parallel agents their own worktree.",
    shot: workspacesShot,
    alt: "Sessions grouped by project with the project picker open",
  },
  {
    tab: "Tasks",
    icon: ListChecks,
    title: "Work that stays claimed until it closes",
    body: "Durable tasks a session claims, heartbeats, and closes with evidence.",
    shot: tasksShot,
    alt: "The Tasks board with pending, blocked and done work",
  },
  {
    tab: "Profiles",
    icon: Users,
    title: "Separate work on one install",
    body: "Each session and automation files under a named profile. Profiles organize work; they are not a security boundary.",
    shot: profilesShot,
    alt: "The profile switcher with several named profiles",
  },
  {
    tab: "Gateway",
    icon: Globe,
    title: "Local until you turn remote on",
    body: "The daemon starts reachable only on this machine. Remote access is off by default.",
    shot: gatewayShot,
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

export const COMPARE_COLUMNS = ["Synara", "T3 Code", "Hermes", "OpenClaw", "DIY stack"];

export const COMPARE_ROWS = [
  "Runs the agents you already use",
  "Always-on agents that check in on their own",
  "One board by who owns the next step",
  "Loops, schedules, and triggers",
  "Approvals and audit",
  "Local-first and MIT",
];

export const INSTALL_METHODS = [
  { id: "Installer", command: INSTALL_COMMAND },
  { id: "Homebrew", command: "brew install compozy/compozy/compozy" },
  { id: "npm", command: "npm install -g @compozy/cli" },
  { id: "Go", command: "go install github.com/compozy/compozy@{tag}" },
];
