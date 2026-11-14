/**
 * "Clean up history" (#1717) as the daemon's dry run and run would report it: one busy project,
 * and an All projects archive past 500 sessions. Shared by the component prototype and tests.
 */
import {
  DEFAULT_CLEANUP_OPTIONS,
  EMPTY_CLEANUP_IMPACT,
  type CleanupChoiceCounts,
  type CleanupImpact,
  type CleanupOptions,
  type CleanupPhase,
} from "../lib/archived-cleanup-model";

const DAY_MS = 24 * 60 * 60 * 1000;
const ONLY = { everything: false, children: false, subagents: false };

export interface ArchivedCleanupFixture {
  scopeName: string;
  acrossProjects?: boolean;
  archivedCount: number;
  options: CleanupOptions;
  impact: CleanupImpact;
  counts: CleanupChoiceCounts;
  phase: CleanupPhase;
  /** The dialog is closed: only the footer shows. */
  closed?: boolean;
}

/** Everything archived 7+ days ago in one project. */
const PROJECT_IMPACT: CleanupImpact = {
  sessions: 41,
  children: 63,
  subagents: 54,
  total: 158,
  keptLive: 2,
  projects: [{ project: "compozy", sessions: 158 }],
};

const PROJECT_COUNTS: CleanupChoiceCounts = { everything: 158, children: 89, subagents: 54 };

const PROJECT: Omit<ArchivedCleanupFixture, "options" | "impact" | "counts" | "phase"> = {
  scopeName: "compozy",
  archivedCount: 214,
};

export const ARCHIVED_CLEANUP_FIXTURES = {
  "archived-list": {
    ...PROJECT,
    options: DEFAULT_CLEANUP_OPTIONS,
    impact: PROJECT_IMPACT,
    counts: PROJECT_COUNTS,
    phase: { kind: "form" },
    closed: true,
  },
  dialog: {
    ...PROJECT,
    options: DEFAULT_CLEANUP_OPTIONS,
    impact: PROJECT_IMPACT,
    counts: PROJECT_COUNTS,
    phase: { kind: "form" },
  },
  "dialog-children": {
    ...PROJECT,
    options: { ...DEFAULT_CLEANUP_OPTIONS, what: { ...ONLY, children: true }, range: "30" },
    impact: { ...EMPTY_CLEANUP_IMPACT, children: 37, subagents: 12, total: 49, keptLive: 1 },
    counts: { everything: 96, children: 49, subagents: 31 },
    phase: { kind: "form" },
  },
  "dialog-subagents": {
    ...PROJECT,
    options: {
      ...DEFAULT_CLEANUP_OPTIONS,
      what: { ...ONLY, children: true, subagents: true },
      range: "15",
    },
    impact: { ...EMPTY_CLEANUP_IMPACT, children: 52, subagents: 47, total: 99, keptLive: 2 },
    counts: { everything: 131, children: 71, subagents: 47 },
    phase: { kind: "form" },
  },
  "dialog-before": {
    ...PROJECT,
    options: {
      ...DEFAULT_CLEANUP_OPTIONS,
      range: "before",
      before: new Date(Date.now() - 20 * DAY_MS),
    },
    impact: {
      ...PROJECT_IMPACT,
      sessions: 30,
      children: 44,
      subagents: 39,
      total: 113,
      keptLive: 0,
    },
    counts: { everything: 113, children: 64, subagents: 39 },
    phase: { kind: "form" },
  },
  "dialog-empty": {
    scopeName: "compozy-site",
    archivedCount: 6,
    options: DEFAULT_CLEANUP_OPTIONS,
    impact: EMPTY_CLEANUP_IMPACT,
    counts: { everything: 0, children: 0, subagents: 0 },
    phase: { kind: "form" },
  },
  deleting: {
    ...PROJECT,
    options: DEFAULT_CLEANUP_OPTIONS,
    impact: PROJECT_IMPACT,
    counts: PROJECT_COUNTS,
    phase: { kind: "deleting", done: 64, total: 158 },
  },
  background: {
    ...PROJECT,
    options: DEFAULT_CLEANUP_OPTIONS,
    impact: PROJECT_IMPACT,
    counts: PROJECT_COUNTS,
    phase: { kind: "deleting", done: 64, total: 158 },
    closed: true,
  },
  partial: {
    ...PROJECT,
    options: DEFAULT_CLEANUP_OPTIONS,
    impact: PROJECT_IMPACT,
    counts: PROJECT_COUNTS,
    phase: {
      kind: "partial",
      total: 158,
      deleted: 147,
      failedThreads: 2,
      failed: [
        {
          sessionId: "sess-1704",
          title: "Fix #1704 cap reasoning effort levels",
          reason: "It is an always-on agent's main session.",
        },
        {
          sessionId: "sess-1702",
          title: "Keep Archived and Later views when opening a session",
          reason: "database is locked",
        },
      ],
    },
  },
  failed: {
    ...PROJECT,
    options: DEFAULT_CLEANUP_OPTIONS,
    impact: PROJECT_IMPACT,
    counts: PROJECT_COUNTS,
    phase: { kind: "failed", reason: "database is locked" },
  },
  scale: {
    scopeName: "All projects",
    acrossProjects: true,
    archivedCount: 512,
    options: { ...DEFAULT_CLEANUP_OPTIONS, range: "all" },
    impact: {
      sessions: 512,
      children: 388,
      subagents: 304,
      total: 1204,
      keptLive: 3,
      projects: [
        { project: "compozy", sessions: 502 },
        { project: "polygo", sessions: 226 },
        { project: "compozy-site", sessions: 168 },
        { project: "branas-dashboard", sessions: 113 },
        { project: "yoshi", sessions: 79 },
        { project: "bible-yt-flow", sessions: 52 },
        { project: "releasepr", sessions: 39 },
        { project: "dotfiles", sessions: 25 },
      ],
    },
    counts: { everything: 1204, children: 692, subagents: 304 },
    phase: { kind: "form" },
  },
} satisfies Record<string, ArchivedCleanupFixture>;

export type ArchivedCleanupFixtureState = keyof typeof ARCHIVED_CLEANUP_FIXTURES;
