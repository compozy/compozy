/** The problem panes and the always-on beats, shared by the pinned stages and their static fallbacks. */
import { CalendarClock, FileText, Hand, Webhook, type LucideIcon } from "@compozy/ui/icons";

import { ASSET } from "@/components/landing/assets";
import type { Focus } from "@/components/site/layout";

export const PANES = [
  {
    id: "waiting",
    name: "claude",
    lines: ["Waiting for your input…", "was this the payments repo?"],
    x: -27,
    y: -21,
    r: -5,
  },
  { id: "approval", name: "codex", lines: ["Allow command? [y/N]", "▌"], x: 26, y: -24, r: 4 },
  {
    id: "editing",
    name: "cursor",
    lines: ["Editing checkout/retry.ts", "Applying 14 changes…"],
    x: -31,
    y: 21,
    r: 3,
  },
  {
    id: "expired",
    name: "claude",
    lines: ["Session expired.", "$ tmux attach -t 4"],
    x: 0,
    y: 27,
    r: -2,
  },
  {
    id: "limited",
    name: "codex",
    lines: ["Rate limited. Retrying in 42s", "$ crontab -e"],
    x: 30,
    y: 19,
    r: -5,
  },
] as const;

export type Pane = (typeof PANES)[number];

export const PILE_LINE = "CompozyOS replaces the pile, not one tool in it.";

export const BEATS: ReadonlyArray<{
  label: string;
  icon: LucideIcon;
  body: string;
  src: string;
  alt: string;
  focus?: Focus;
}> = [
  {
    label: "Check-ins",
    icon: CalendarClock,
    body: "On a schedule, the agent reviews its project in a hidden, read-only session. A quiet check costs no model call.",
    src: ASSET.heartbeat,
    alt: "An agent's heartbeat page: next check, today's quiet checks, and why each one stayed silent",
  },
  {
    label: "Sources",
    icon: Webhook,
    body: "Point it at what it should watch: new GitHub issues from an extension, or a script you approve.",
    src: ASSET.heartbeat,
    alt: "An agent's Sources list watching new GitHub issues in a repository",
    focus: { x: 0.14, y: 0.31, w: 0.46 },
  },
  {
    label: "Reports",
    icon: FileText,
    body: "Reports land in its main session. Come back after a day away and one card sums up what happened.",
    src: ASSET.reports,
    alt: "A While you were away card: ten quiet hours, two decisions, with a check-in timeline",
  },
  {
    label: "Proposals",
    icon: Hand,
    body: "Pick a proposed action, or let the agent do it without asking, up to a daily limit you set.",
    src: ASSET.proposal,
    alt: "Two proposals from the agent, each waiting for someone to pick it",
  },
];

export const FACES = [
  "ci-doctor",
  "infra-drift",
  "pr-shepherd",
  "risk-monitor",
  "release-watch",
  "log-triage",
  "dependency-watch",
  "issue-triager",
  "orchestrator",
  "runbook-keeper",
  "security-scan",
  "ux-regression",
  "checkout-engineer",
  "docs-steward",
];
