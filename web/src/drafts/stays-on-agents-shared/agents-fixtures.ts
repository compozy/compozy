import type { ListedAgent } from "./agents-page";
import {
  CODE_ENGINEER,
  GENERAL,
  ORCHESTRATOR,
  RELEASE,
  REVIEWER,
  SCOUT,
  TRIAGE,
  type DraftAgent,
  type PauseReason,
} from "./fixtures";

/** Why it stopped, the way you'd say it to a colleague. */
export const PAUSE_PLAIN: Record<PauseReason, string> = {
  manual: "You paused it.",
  fatigue: "It stopped itself after 10 check-ins in a row went wrong.",
  definition_not_stays_on: "It was changed to on demand, so it can't stay on here anymore.",
  definition_missing: "Its setup was deleted, so it can't come back. What it remembers is kept.",
};

/** Fifty real-looking projects, so filters and "where it works" are tested at scale. */
/** The same reason, short enough for a card. */
export const PAUSE_SHORT: Record<PauseReason, string> = {
  manual: "You paused it",
  fatigue: "Stopped after 10 failed check-ins",
  definition_not_stays_on: "Now on demand, so it can't stay on",
  definition_missing: "Its setup was deleted",
};

const PROJECT_STEMS = [
  "compozy",
  "reloaddev",
  "advos",
  "billing-api",
  "checkout-web",
  "docs-site",
  "mobile-app",
  "data-pipeline",
  "design-system",
  "support-portal",
] as const;
const PROJECT_TAILS = ["", "-staging", "-eu", "-legacy", "-labs"] as const;

export const PROJECTS: readonly string[] = PROJECT_TAILS.flatMap(tail =>
  PROJECT_STEMS.map(stem => `${stem}${tail}`)
);

const ACTIVITY = {
  busy: [6, 9, 7, 12, 14, 4, 3, 8, 11, 13, 15, 12, 5, 9],
  steady: [4, 5, 4, 6, 5, 2, 2, 5, 4, 6, 5, 6, 3, 4],
  light: [0, 1, 0, 2, 0, 0, 0, 1, 3, 0, 1, 2, 0, 1],
  fading: [8, 7, 9, 6, 5, 4, 3, 2, 1, 0, 0, 0, 0, 0],
  none: [],
} as const;

const ORCHESTRATOR_LISTED: ListedAgent = {
  ...ORCHESTRATOR,
  places: [
    { scope: "compozy", state: "working", now: "Handing out the next 3 tasks" },
    { scope: "reloaddev", state: "idle" },
  ],
  activity: ACTIVITY.busy,
  recap: "118 reports in 2 weeks",
  lastActive: "Now",
};

const TRIAGE_LISTED: ListedAgent = {
  ...TRIAGE,
  places: [
    { scope: "compozy", state: "working", now: "Sorting 14 new issues" },
    { scope: "advos", state: "idle" },
  ],
  activity: ACTIVITY.steady,
  recap: "61 reports in 2 weeks",
  lastActive: "Now",
};

const RELEASE_LISTED: ListedAgent = {
  ...RELEASE,
  places: [{ scope: "@user", state: "idle" }],
  activity: ACTIVITY.light,
  recap: "11 reports in 2 weeks",
  lastActive: "22 min ago",
};

const SCOUT_LISTED: ListedAgent = {
  ...SCOUT,
  places: [],
  activity: ACTIVITY.none,
  lastActive: undefined,
};

export const LISTED_STAYS_ON: readonly ListedAgent[] = [
  ORCHESTRATOR_LISTED,
  TRIAGE_LISTED,
  RELEASE_LISTED,
  SCOUT_LISTED,
];

export const LISTED_ON_DEMAND: readonly ListedAgent[] = [
  { ...GENERAL, activity: ACTIVITY.steady, recap: "128 sessions", lastActive: "Now" },
  { ...CODE_ENGINEER, activity: ACTIVITY.busy, recap: "64 sessions", lastActive: "Now" },
  { ...REVIEWER, activity: ACTIVITY.light, recap: "37 sessions", lastActive: "2 days ago" },
];

export const LISTED_PAUSED: readonly ListedAgent[] = [
  ORCHESTRATOR_LISTED,
  {
    ...TRIAGE_LISTED,
    places: [
      { scope: "compozy", state: "paused", pauseReason: "manual" },
      { scope: "advos", state: "paused", pauseReason: "manual" },
    ],
    activity: ACTIVITY.fading,
    lastActive: "5 days ago",
  },
  {
    ...RELEASE_LISTED,
    places: [{ scope: "@user", state: "paused", pauseReason: "fatigue" }],
    activity: ACTIVITY.fading,
    lastActive: "Yesterday",
  },
  {
    ...SCOUT_LISTED,
    places: [{ scope: "reloaddev", state: "paused", pauseReason: "definition_missing" }],
    activity: ACTIVITY.fading,
    lastActive: "3 days ago",
  },
];

export const LISTED_NEEDS_YOU: readonly ListedAgent[] = [
  {
    ...TRIAGE_LISTED,
    places: [
      { scope: "compozy", state: "needs-you", now: "Wants your OK to close 3 duplicate issues" },
      { scope: "advos", state: "idle" },
    ],
  },
  ORCHESTRATOR_LISTED,
  RELEASE_LISTED,
];

export const LISTED_SAME_SLUG: readonly ListedAgent[] = [
  {
    ...LISTED_ON_DEMAND[0]!,
    origin: "compozy",
    scopes: ["compozy"],
    active: 2,
  },
  {
    ...LISTED_ON_DEMAND[0]!,
    description: "Answers questions about the reloaddev deploy pipeline",
    origin: "reloaddev",
    provider: "codex",
    scopes: ["reloaddev"],
    active: 0,
    lastActive: "4 days ago",
  },
  LISTED_ON_DEMAND[1]!,
  LISTED_ON_DEMAND[2]!,
];

export const LISTED_INVALID: readonly ListedAgent[] = [
  LISTED_ON_DEMAND[1]!,
  {
    ...LISTED_ON_DEMAND[2]!,
    invalid: true,
    description: "Its setup file has a mistake, so it can't start until that's fixed.",
    lastActive: undefined,
    recap: undefined,
    activity: ACTIVITY.none,
  },
  LISTED_ON_DEMAND[0]!,
];

const ROLES = [
  { role: "Reviewer", job: "Reads every change to {area} and says what would break" },
  { role: "Fixer", job: "Fixes small bugs in {area} and opens the pull request" },
  { role: "Test Writer", job: "Adds the tests {area} is missing" },
  { role: "Docs Keeper", job: "Keeps the {area} guides true to what shipped" },
  { role: "Bug Hunter", job: "Reproduces reported bugs in {area} step by step" },
  { role: "Migration Pilot", job: "Moves {area} to the new version, one safe step at a time" },
  { role: "Copy Editor", job: "Tidies the words people read in {area}" },
  { role: "Spec Author", job: "Turns rough ideas for {area} into a clear plan" },
  { role: "Issue Sorter", job: "Labels what comes in for {area} and sends it to the right place" },
  { role: "Release Notes Writer", job: "Writes what changed in {area} in plain words" },
] as const;

const AREAS = [
  "Billing",
  "Checkout",
  "Docs",
  "Mobile",
  "Search",
  "Auth",
  "Onboarding",
  "Analytics",
  "Support",
  "Data",
] as const;

const SHAPES = [ACTIVITY.busy, ACTIVITY.steady, ACTIVITY.light, ACTIVITY.fading];
const WHEN = ["1 hour ago", "Yesterday", "3 days ago", "Last week", "2 weeks ago"] as const;

function generated(index: number, base: DraftAgent, ownerless = true): ListedAgent {
  const role = ROLES[index % ROLES.length]!;
  const area = AREAS[Math.floor(index / ROLES.length) % AREAS.length]!;
  const label = `${area} ${role.role}`;
  const name = label.toLowerCase().replace(/\s/g, "-");
  return {
    ...base,
    name,
    label,
    description: role.job.replace("{area}", area),
    appearance: { avatar: "generated", seed: name },
    provider: index % 3 === 0 ? "codex" : "claude",
    model: index % 3 === 0 ? "gpt-6-astra" : "claude-sonnet-5-5",
    owner: ownerless ? undefined : "spec-cycle",
    scopes: PROJECTS.slice(index % 40, (index % 40) + (index % 4) + 1),
    active: 0,
    activity: SHAPES[index % SHAPES.length],
    recap: `${((index * 7) % 90) + 4} sessions`,
    lastActive: WHEN[index % WHEN.length],
  };
}

/** Twelve stays-on agents with 50 projects between them, one with a very long name. */
export const MANY_STAYS_ON: readonly ListedAgent[] = [
  LISTED_NEEDS_YOU[0]!,
  ORCHESTRATOR_LISTED,
  RELEASE_LISTED,
  {
    ...generated(1, TRIAGE),
    label: "Customer Support Escalations Reviewer for Enterprise Accounts in Europe",
    places: PROJECTS.map((scope, index) => ({
      scope,
      state: index === 3 ? ("working" as const) : ("idle" as const),
      now: index === 3 ? "Answering 2 waiting tickets" : undefined,
    })),
    lastActive: "Now",
    recap: "340 reports in 2 weeks",
  },
  ...[11, 22, 33, 44, 55, 66, 77, 88].map(index => {
    const agent = generated(index, TRIAGE);
    return {
      ...agent,
      recap: `${index} reports in 2 weeks`,
      places: agent.scopes.slice(0, 3).map(scope => ({ scope, state: "idle" as const })),
    };
  }),
];

/** The on-demand rest of a 100-agent team. */
export const MANY_ON_DEMAND: readonly ListedAgent[] = [
  ...LISTED_ON_DEMAND,
  ...Array.from({ length: 100 }, (_, index) => index)
    .filter(index => index % 11 !== 0)
    .slice(0, 85)
    .map(index => generated(index, CODE_ENGINEER, index % 9 !== 0)),
];

/** One team member's profile state. */
export function profileStateOf(agent: ListedAgent) {
  if (agent.invalid) return "invalid";
  if (agent.kind === "on_demand") return "on-demand";
  if (agent.owner) return "extension";
  const places = agent.places ?? [];
  if (places.length === 0) return "no-activity";
  if (places.some(place => place.state === "needs-you")) return "needs-you";
  if (places.some(place => place.pauseReason === "definition_missing")) return "cant-resume";
  if (places.some(place => place.pauseReason === "fatigue")) return "paused-failures";
  if (places.every(place => place.state === "paused")) return "paused";
  return "stays-on";
}
