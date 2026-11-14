/**
 * The archive the "Clean up history" draft (#1717) previews: one busy project, and an All projects
 * archive past 500 sessions across many projects and folders.
 */
import { primarySessionFixture } from "@/systems/session/mocks/fixtures";
import { sessionRuntimeWithModel } from "@/systems/session/mocks/runtime-fixture";
import type { SessionPayload } from "@/systems/session/types";

import type { ArchivedThread } from "./archived-cleanup-model";

const NOW = Date.now();
const DAY_MINUTES = 24 * 60;
const ago = (minutes: number) => new Date(NOW - minutes * 60_000).toISOString();

const TITLES = [
  "Fix #1704 cap reasoning effort levels",
  "Session tabs show the agent avatar",
  "Keep Archived and Later views when opening a session",
  "Stop reporting a deleted child as working",
  "Remove idle shell subscriptions from tab switching",
  "Files preview: notebooks and PDFs",
  "Review PR #1694 files preview",
  "Release v0.7.0 merge train",
  "MCP hot activation spec",
  "Investigate daemon CPU on catalog reads",
  "Window tabs and splits QA walk",
  "Appearance text size and spacing",
  "Marketplace plugin catalog icons",
  "Brand site inner pages",
  "Terminal login shell on macOS",
  "Kanban Waiting column fold",
  "Session menus parity audit",
  "Provider usage collection",
  "Tasks removal — slice 3",
  "Composer @-mentions for folders",
] as const;

const AGENTS = ["code_implementer", "orchestrator", "reviewer", "general", "spec_conductor"];

/** A small deterministic spread so counts look like real work, not a pattern. */
function spread(index: number, mod: number, salt: number): number {
  return (index * 7 + salt * 13 + ((index * index) % 11)) % mod;
}

function thread(index: number, project: string, archivedDaysAgo: number): ArchivedThread {
  const orchestrated = index % 4 === 1;
  return {
    id: `arch-${project}-${index}`,
    title: TITLES[index % TITLES.length]!,
    project,
    archivedDaysAgo,
    children: orchestrated ? 2 + spread(index, 4, 1) : index % 5 === 0 ? 1 : 0,
    subagents: spread(index, 9, 2),
    liveChildren: orchestrated && index % 8 === 1 ? 1 : 0,
  };
}

/**
 * One project's archive, 34 main sessions over two months: a handful last week, more the week
 * before, a long tail after.
 */
const PROJECT_DAYS = [
  0, 0, 1, 2, 3, 4, 6, 9, 10, 12, 13, 14, 17, 19, 21, 23, 25, 26, 28, 29, 33, 36, 39, 41, 44, 47,
  50, 52, 55, 57, 58, 60, 61, 63,
];

export const PROJECT_ARCHIVE: readonly ArchivedThread[] = PROJECT_DAYS.map((days, index) =>
  thread(index, "compozy", days)
);

/** Every project of the profile, busiest first: 512 archived main sessions in all. */
const SCALE_PROJECTS: readonly { project: string; count: number }[] = [
  { project: "compozy", count: 214 },
  { project: "polygo", count: 96 },
  { project: "compozy-site", count: 71 },
  { project: "branas-dashboard", count: 48 },
  { project: "yoshi", count: 33 },
  { project: "bible-yt-flow", count: 22 },
  { project: "releasepr", count: 17 },
  { project: "dotfiles", count: 11 },
];

export const SCALE_ARCHIVE: readonly ArchivedThread[] = SCALE_PROJECTS.flatMap(
  ({ project, count }, projectIndex) =>
    Array.from({ length: count }, (_, index) =>
      thread(index + projectIndex, project, spread(index, 120, projectIndex))
    )
);

export const SCALE_ARCHIVE_TOTAL = SCALE_ARCHIVE.length;

export const SCALE_PROJECT_NAMES = SCALE_PROJECTS.map(item => item.project);

/** A list row for an archived main session. */
export function archivedRow(
  item: ArchivedThread,
  overrides: Partial<SessionPayload> = {}
): SessionPayload {
  const index = Number(item.id.split("-").pop());
  return {
    ...primarySessionFixture,
    id: item.id,
    name: item.title,
    agent_name: AGENTS[index % AGENTS.length]!,
    runtime: sessionRuntimeWithModel(index % 3 === 2 ? "codex" : "claude"),
    state: "stopped",
    badge: "done",
    archived_at: ago(item.archivedDaysAgo * DAY_MINUTES + 40),
    updated_at: ago(item.archivedDaysAgo * DAY_MINUTES + 95),
    pull_request: null,
    linked_work: null,
    later: undefined,
    subagent_summary:
      item.subagents > 0
        ? {
            total: item.subagents,
            live: 0,
            failed: 0,
            attention: 0,
            most_urgent: "done",
          }
        : undefined,
    ...overrides,
  };
}

/** The rows the Archived list shows first: the most recently archived. */
export function archivedRows(archive: readonly ArchivedThread[], limit: number): SessionPayload[] {
  const seen = new Set<string>();
  return [...archive]
    .sort((a, b) => a.archivedDaysAgo - b.archivedDaysAgo)
    .filter(item => !seen.has(item.title) && Boolean(seen.add(item.title)))
    .slice(0, limit)
    .map(item => archivedRow(item));
}
