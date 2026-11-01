/** What the Activity page reads: permission entries, each naming who started the work. */
import type { LucideIcon } from "lucide-react";
import { FileEdit, FilePlus2, FolderSearch, Globe, SquareTerminal } from "lucide-react";

import {
  CODE_ENGINEER,
  GENERAL,
  ORCHESTRATOR,
  REVIEWER,
  TRIAGE,
  type DraftAgent,
} from "../stays-on-agents-shared/fixtures";

export type Starter = "human" | "automation" | "daemon" | "extension" | "unknown";
export type Result = "allowed" | "declined" | "expired";

export interface ActivityEntry {
  id: string;
  day: "Today" | "Yesterday" | "Oct 12";
  when: string;
  icon: LucideIcon;
  action: string;
  detail: string;
  who: DraftAgent;
  starter: Starter;
  /** The automation or extension by name, when there is one. */
  source?: string;
  result: Result;
}

export const STARTER_LABEL: Record<Starter, string> = {
  human: "You",
  automation: "Automation",
  daemon: "Check-in",
  extension: "Extension",
  unknown: "Unknown",
};

export const UNATTENDED: ReadonlySet<Starter> = new Set(["automation", "daemon", "extension"]);

export const ENTRIES: readonly ActivityEntry[] = [
  {
    id: "1",
    day: "Today",
    when: "04:12",
    icon: SquareTerminal,
    action: "Ran the test suite",
    detail: "bun test",
    who: REVIEWER,
    starter: "automation",
    source: "Nightly review",
    result: "allowed",
  },
  {
    id: "2",
    day: "Today",
    when: "03:58",
    icon: FolderSearch,
    action: "Read the task store",
    detail: "internal/task/store.go",
    who: TRIAGE,
    starter: "daemon",
    result: "allowed",
  },
  {
    id: "3",
    day: "Today",
    when: "02:14",
    icon: FileEdit,
    action: "Wanted to edit the boot file",
    detail: "internal/daemon/boot.go",
    who: CODE_ENGINEER,
    starter: "daemon",
    result: "expired",
  },
  {
    id: "4",
    day: "Today",
    when: "01:30",
    icon: Globe,
    action: "Looked up the release notes",
    detail: "github.com/compozy/compozy",
    who: ORCHESTRATOR,
    starter: "extension",
    source: "spec-cycle",
    result: "allowed",
  },
  {
    id: "5",
    day: "Today",
    when: "00:47",
    icon: SquareTerminal,
    action: "Wanted to run the migration",
    detail: "make migrate",
    who: CODE_ENGINEER,
    starter: "automation",
    source: "Token rollout",
    result: "declined",
  },
  {
    id: "6",
    day: "Yesterday",
    when: "23:41",
    icon: FolderSearch,
    action: "Read the open issues",
    detail: "github.com/compozy/compozy/issues",
    who: TRIAGE,
    starter: "daemon",
    result: "allowed",
  },
  {
    id: "7",
    day: "Yesterday",
    when: "22:12",
    icon: FileEdit,
    action: "Edited the app shell",
    detail: "web/src/app.tsx",
    who: GENERAL,
    starter: "human",
    result: "allowed",
  },
  {
    id: "8",
    day: "Yesterday",
    when: "21:02",
    icon: FilePlus2,
    action: "Wrote the release notes",
    detail: "docs/notes.md",
    who: ORCHESTRATOR,
    starter: "extension",
    source: "spec-cycle",
    result: "allowed",
  },
];

export const LEGACY: ActivityEntry = {
  id: "9",
  day: "Oct 12",
  when: "09:20",
  icon: SquareTerminal,
  action: "Ran the test suite",
  detail: "bun test",
  who: GENERAL,
  starter: "unknown",
  result: "allowed",
};

/** Fourteen days, shaped like work that runs at night and pauses at the weekend. */
export const DAYS = [
  [38, 1, 0],
  [41, 0, 1],
  [12, 0, 0],
  [9, 0, 0],
  [44, 2, 1],
  [52, 1, 0],
  [47, 0, 2],
  [39, 1, 0],
  [46, 0, 0],
  [15, 0, 0],
  [11, 1, 0],
  [58, 2, 1],
  [49, 0, 0],
  [36, 1, 1],
].map(([allowed, declined, expired], index) => ({
  date: `Oct ${index + 18}`,
  allowed: allowed!,
  declined: declined!,
  expired: expired!,
}));

const NIGHT_HOURS = [22, 23, 0, 1, 2, 3, 4, 5, 9, 14];
const WEEKEND = new Set([0, 6]);

/** Most of it lands between ten at night and six in the morning, and less at the weekend. */
export const PULSE = {
  window_days: 14,
  buckets: [1, 2, 3, 4, 5, 6, 0].flatMap((weekday, row) =>
    NIGHT_HOURS.flatMap((hour, index) => {
      const seed = (row * 7 + index * 13 + row * index) % 11;
      if (seed === 0 || (WEEKEND.has(weekday) && seed < 5)) return [];
      return [{ weekday, hour, events: (WEEKEND.has(weekday) ? 2 : 6) + seed * 2 }];
    })
  ),
  busiest: { weekday: 3, hour: 1, events: 26 },
};

export const STATS = {
  unattended: {
    total: 455,
    allowed: 437,
    declined: 9,
    expired: 9,
    sentence: "455 actions ran with nobody in the conversation.",
    starters: [
      { key: "daemon", label: "Check-ins", value: 248 },
      { key: "automation", label: "Automations", value: 151 },
      { key: "extension", label: "Extensions", value: 56 },
    ],
  },
  all: {
    total: 1204,
    allowed: 1176,
    declined: 19,
    expired: 9,
    sentence: "1,204 actions in the last 14 days.",
    starters: [
      { key: "human", label: "You", value: 749 },
      { key: "daemon", label: "Check-ins", value: 248 },
      { key: "automation", label: "Automations", value: 151 },
      { key: "extension", label: "Extensions", value: 56 },
    ],
  },
} as const;

export const AGENT_SHARE = [
  { agent: TRIAGE, value: 212 },
  { agent: ORCHESTRATOR, value: 131 },
  { agent: CODE_ENGINEER, value: 74 },
  { agent: REVIEWER, value: 38 },
];
