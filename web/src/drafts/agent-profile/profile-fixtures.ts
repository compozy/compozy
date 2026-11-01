/** Every profile the prototype can show, from a real day's worth of made-up work. */
import type { AgentState } from "../persistent-agents-shared/agent-state";
import { PROJECTS } from "../persistent-agents-shared/agents-fixtures";
import {
  GENERAL,
  ORCHESTRATOR,
  SCOUT,
  TRIAGE,
  type DraftAgent,
  type PauseReason,
} from "../persistent-agents-shared/fixtures";

export interface ProfilePlace {
  scope: string;
  state: AgentState;
  pauseReason?: PauseReason;
  now?: string;
  last?: string;
  next?: string;
  activity: readonly number[];
}

export interface Finding {
  id: string;
  title: string;
  scope: string;
  when: string;
  /** Waiting for a decision from you. */
  waiting?: boolean;
}

export interface WorkItem {
  id: string;
  title: string;
  detail: string;
  when: string;
  tone?: "neutral" | "success" | "warning" | "danger";
}

export interface DayActivity {
  date: string;
  quiet: number;
  reported: number;
  needed: number;
}

export interface ProfileData {
  agent: DraftAgent;
  places: readonly ProfilePlace[];
  days: readonly DayActivity[];
  findings: readonly Finding[];
  work: readonly WorkItem[];
  usage: { cost: string; tokens: string; days: readonly { date: string; value: number }[] };
  usageByProject: readonly { project: string; share: number; amount: string }[];
  pulse: readonly { weekday: number; hour: number; events: number }[];
  /** Shown in the main stat beside the chart. */
  headline: { value: string; unit: string; sub: string };
  readOnly?: boolean;
  fresh?: boolean;
}

function day(offset: number) {
  const date = new Date(Date.UTC(2026, 9, 2 + offset));
  return date.toLocaleDateString("en-US", { day: "numeric", month: "short", timeZone: "UTC" });
}

/** Thirty days that rise and fall the way a working month does: quiet weekends, busy mid-weeks. */
function month(scale: number, tail = 1): DayActivity[] {
  return Array.from({ length: 30 }, (_, index) => {
    const weekend = index % 7 === 2 || index % 7 === 3;
    const wave = 3 + ((index * 7) % 5);
    const fade = index > 30 - 8 ? tail : 1;
    return {
      date: day(index),
      quiet: Math.round((weekend ? 6 : 12 + wave) * scale * fade),
      reported: Math.round((weekend ? 1 : wave) * scale * fade),
      needed: Math.round(index % 6 === 1 ? 2 * scale * fade : 0),
    };
  });
}

function spend(scale: number, tail = 1) {
  return Array.from({ length: 30 }, (_, index) => ({
    date: day(index),
    value: Math.round(
      (index % 7 === 2 || index % 7 === 3 ? 30 : 90 + ((index * 13) % 70)) *
        1000 *
        scale *
        (index > 22 ? tail : 1)
    ),
  }));
}

function rhythm(start: number, end: number, scale: number) {
  return [1, 2, 3, 4, 5].flatMap(weekday =>
    Array.from({ length: end - start }, (_, index) => ({
      weekday,
      hour: start + index,
      events: Math.round((((index * 3 + weekday * 5) % 7) + 2) * scale),
    }))
  );
}

const BUSY = [6, 9, 7, 12, 14, 4, 3, 8, 11, 13, 15, 12, 5, 9];
const STEADY = [4, 5, 4, 6, 5, 2, 2, 5, 4, 6, 5, 6, 3, 4];
const STOPPED = [8, 7, 9, 6, 5, 4, 3, 2, 1, 0, 0, 0, 0, 0];

const COMPOZY: ProfilePlace = {
  scope: "compozy",
  state: "working",
  now: "Sorting 14 new issues",
  last: "12 min ago",
  next: "in 18 min",
  activity: BUSY,
};
const ADVOS: ProfilePlace = {
  scope: "advos",
  state: "idle",
  last: "31 min ago",
  next: "in 29 min",
  activity: STEADY,
};

const FINDINGS: readonly Finding[] = [
  {
    id: "f1",
    title: "3 issues are duplicates of #1102",
    scope: "compozy",
    when: "4 min ago",
    waiting: false,
  },
  {
    id: "f2",
    title: "The login bug is back after Tuesday's release",
    scope: "advos",
    when: "2 hours ago",
  },
  { id: "f3", title: "Nobody has answered #1087 in 6 days", scope: "compozy", when: "Yesterday" },
  { id: "f4", title: "Two people asked for dark mode exports", scope: "compozy", when: "Monday" },
];

const WORK: readonly WorkItem[] = [
  {
    id: "w1",
    title: "Sorted 22 new issues",
    detail: "Labeled 19, sent 3 to you",
    when: "12 min ago",
    tone: "success",
  },
  { id: "w2", title: "Checked in, nothing new", detail: "advos was quiet", when: "31 min ago" },
  {
    id: "w3",
    title: "Proposed splitting #1102",
    detail: "You said yes",
    when: "Today, 9:40",
    tone: "success",
  },
  {
    id: "w4",
    title: "Couldn't reach GitHub",
    detail: "Tried again 5 min later and it worked",
    when: "Yesterday",
    tone: "warning",
  },
  {
    id: "w5",
    title: "Wrote the weekly summary",
    detail: "Sent to your main session",
    when: "Monday",
  },
];

const USAGE_SPLIT = [
  { project: "compozy", share: 68, amount: "≈ $8.40" },
  { project: "advos", share: 32, amount: "≈ $4.00" },
];

const STAY_ON: Omit<ProfileData, "places"> = {
  agent: TRIAGE,
  days: month(1),
  findings: FINDINGS,
  work: WORK,
  usage: { cost: "≈ $12.40", tokens: "3.1M tokens", days: spend(1) },
  usageByProject: USAGE_SPLIT,
  pulse: rhythm(9, 19, 1),
  headline: { value: "186", unit: "reports", sub: "6 a day. 22 more than last month" },
};

const PAUSED_SINCE = {
  days: month(1, 0),
  usage: { cost: "≈ $9.10", tokens: "2.2M tokens", days: spend(1, 0) },
};

function pausedPlace(reason: PauseReason): ProfilePlace {
  return {
    ...COMPOZY,
    state: "paused",
    pauseReason: reason,
    now: undefined,
    next: undefined,
    last: "2 days ago",
    activity: STOPPED,
  };
}

const MANY_PLACES: readonly ProfilePlace[] = PROJECTS.map((scope, index) => ({
  scope,
  state:
    index === 0 ? "needs-you" : index % 9 === 1 ? "working" : index % 13 === 4 ? "paused" : "idle",
  pauseReason: index % 13 === 4 ? "manual" : undefined,
  now:
    index === 0 ? "Wants your OK on 3 issues" : index % 9 === 1 ? "Reading new issues" : undefined,
  last: `${((index * 3) % 50) + 2} min ago`,
  next: index % 13 === 4 ? undefined : `in ${((index * 7) % 28) + 2} min`,
  activity: index % 2 === 0 ? BUSY : STEADY,
}));

export const PROFILES = {
  persistent: { ...STAY_ON, places: [COMPOZY, ADVOS] },
  "needs-you": {
    ...STAY_ON,
    places: [
      {
        ...COMPOZY,
        state: "needs-you",
        now: "Wants your OK to close 3 duplicate issues",
        last: "4 min ago",
      },
      ADVOS,
    ],
    findings: [{ ...FINDINGS[0]!, waiting: true }, ...FINDINGS.slice(1)],
  },
  paused: {
    ...STAY_ON,
    ...PAUSED_SINCE,
    places: [
      pausedPlace("manual"),
      { ...ADVOS, state: "paused", pauseReason: "manual", next: undefined, activity: STOPPED },
    ],
  },
  "paused-failures": {
    ...STAY_ON,
    ...PAUSED_SINCE,
    places: [pausedPlace("fatigue"), ADVOS],
    work: [
      {
        id: "w0",
        title: "Stopped checking in on compozy",
        detail: "10 check-ins in a row went wrong",
        when: "2 days ago",
        tone: "danger",
      },
      ...WORK.slice(3),
    ],
  },
  "cant-resume": { ...STAY_ON, ...PAUSED_SINCE, places: [pausedPlace("definition_missing")] },
  "no-longer-stay-on": {
    ...STAY_ON,
    ...PAUSED_SINCE,
    places: [pausedPlace("definition_not_persistent")],
  },
  ephemeral: {
    ...STAY_ON,
    agent: GENERAL,
    places: [],
    findings: [],
    days: month(0.4),
    headline: { value: "128", unit: "sessions", sub: "4 a day. 96% finished without help" },
    usage: { cost: "Included", tokens: "9.8M tokens", days: spend(2.4) },
    usageByProject: [
      { project: "compozy", share: 54, amount: "5.3M" },
      { project: "advos", share: 31, amount: "3.0M" },
      { project: "reloaddev", share: 15, amount: "1.5M" },
    ],
    work: [
      {
        id: "s1",
        title: "Explained how the deploy works",
        detail: "compozy, 4 min",
        when: "1 hour ago",
        tone: "success",
      },
      {
        id: "s2",
        title: "Cleaned up the release checklist",
        detail: "advos, 22 min",
        when: "Today, 10:12",
        tone: "success",
      },
      {
        id: "s3",
        title: "Stopped before it finished",
        detail: "reloaddev, you stopped it",
        when: "Yesterday",
        tone: "warning",
      },
    ],
    pulse: rhythm(10, 17, 0.7),
  },
  extension: {
    ...STAY_ON,
    agent: ORCHESTRATOR,
    readOnly: true,
    usageByProject: [
      { project: "compozy", share: 74, amount: "≈ $9.20" },
      { project: "reloaddev", share: 26, amount: "≈ $3.20" },
    ],
    places: [
      { ...COMPOZY, now: "Handing out the next 3 tasks" },
      { ...ADVOS, scope: "reloaddev" },
    ],
  },
  "no-activity": {
    ...STAY_ON,
    agent: SCOUT,
    places: [],
    days: [],
    findings: [],
    work: [],
    usageByProject: [],
    pulse: [],
    fresh: true,
  },
  invalid: { ...STAY_ON, ...PAUSED_SINCE, places: [pausedPlace("manual")] },
  "many-places": {
    ...STAY_ON,
    places: MANY_PLACES,
    headline: { value: "2,914", unit: "reports", sub: "97 a day across 50 projects" },
  },
} satisfies Record<string, ProfileData>;

export type ProfileId = keyof typeof PROFILES;
