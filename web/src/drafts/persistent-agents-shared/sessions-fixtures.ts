import {
  INSTANCES,
  ORCHESTRATOR,
  PAUSE_COPY,
  RELEASE,
  SCOUT,
  TRIAGE,
  type DraftInstance,
} from "./fixtures";
import { SESSIONS_BOARD_SESSIONS } from "@/systems/session/mocks/sessions-board-fixtures";

export const BOARD_SESSIONS = Object.values(SESSIONS_BOARD_SESSIONS).flat();

export function boardSession(id: string) {
  const session = BOARD_SESSIONS.find(item => item.id === id);
  if (!session) throw new Error(`Missing board fixture "${id}"`);
  return session;
}

/** Sessions that a persistent agent spawned, with the agent that started them. */
export const SPAWNED_BY = {
  s2: INSTANCES.triageCompozy,
  s4: INSTANCES.orchestratorCompozy,
  s9: INSTANCES.orchestratorCompozy,
} as Record<string, DraftInstance | undefined>;

export const COMPOZY_ROW: readonly DraftInstance[] = [
  INSTANCES.orchestratorCompozy,
  INSTANCES.triageCompozy,
];
export const RELOADDEV_ROW: readonly DraftInstance[] = [INSTANCES.orchestratorReloaddev];
export const ADVOS_ROW: readonly DraftInstance[] = [INSTANCES.triageAdvos];
export const GLOBAL_ROW: readonly DraftInstance[] = [INSTANCES.releaseGlobal];

function extra(
  id: string,
  agent: DraftInstance["agent"],
  state: DraftInstance["state"]
): DraftInstance {
  return { id, agent, scope: "compozy", state };
}

/** One avatar per state, for the row's review state. */
export const EACH_STATE: readonly DraftInstance[] = [
  extra("e-working", ORCHESTRATOR, "working"),
  extra("e-needs", TRIAGE, "needs-you"),
  extra("e-failed", RELEASE, "failed"),
  { ...extra("e-paused", SCOUT, "paused"), pauseReason: "fatigue" },
  extra("e-idle", { ...TRIAGE, appearance: { avatar: "generated", seed: "docs" } }, "idle"),
  extra(
    "e-starting",
    { ...TRIAGE, appearance: { avatar: "generated", seed: "qa-lead" } },
    "starting"
  ),
];

export const CROWDED: readonly DraftInstance[] = [
  ...COMPOZY_ROW,
  ...EACH_STATE.slice(2),
  extra(
    "x-1",
    {
      ...ORCHESTRATOR,
      label: "Migration Pilot",
      appearance: { avatar: "generated", seed: "migration-pilot" },
    },
    "idle"
  ),
  extra(
    "x-2",
    {
      ...ORCHESTRATOR,
      label: "Docs Keeper",
      appearance: { avatar: "generated", seed: "docs-keeper" },
    },
    "idle"
  ),
];

function crewAgent(label: string, seed: string, provider = "claude"): DraftInstance["agent"] {
  return {
    ...ORCHESTRATOR,
    name: seed,
    label,
    provider,
    owner: undefined,
    appearance: { avatar: "generated", seed },
  };
}

/** Twelve agents in one project: the row's crowded case, every state present. */
export const CREW: readonly DraftInstance[] = [
  INSTANCES.triageCompozy,
  extra("c-release", RELEASE, "failed"),
  INSTANCES.orchestratorCompozy,
  extra("c-migration", crewAgent("Migration Pilot", "migration-pilot", "codex"), "working"),
  extra("c-qa", crewAgent("QA Lead", "qa-lead"), "working"),
  extra("c-docs", crewAgent("Docs Keeper", "docs-keeper"), "idle"),
  extra("c-deps", crewAgent("Dependency Watch", "dep-watch", "codex"), "idle"),
  extra("c-design", crewAgent("Design Steward", "design-steward"), "idle"),
  extra("c-changelog", crewAgent("Changelog Writer", "changelog"), "idle"),
  extra("c-security", crewAgent("Security Sentry", "sentry", "codex"), "starting"),
  { ...extra("c-scout", SCOUT, "paused"), pauseReason: "fatigue" },
  {
    ...extra("c-support", crewAgent("Support Scribe", "support"), "paused"),
    pauseReason: "manual",
  },
];

/** A second agent asking at once, for the row's "more need you" line. */
export const TWO_NEED_YOU: readonly DraftInstance[] = [
  ...COMPOZY_ROW,
  extra("t-release", RELEASE, "needs-you"),
];

/**
 * What each agent is up to, in a person's words. The instance payload has no such line yet: it
 * needs the main session's latest activity, or the same words the session row already shows.
 */
const DOING: Record<string, string> = {
  [INSTANCES.orchestratorCompozy.id]: "Splitting the billing work into four tasks",
  [INSTANCES.triageCompozy.id]: "Has 2 suggestions for you",
  [INSTANCES.orchestratorReloaddev.id]: "Nothing to do right now",
  [INSTANCES.releaseGlobal.id]: "Nothing to do right now",
  "c-release": "Couldn't finish its last check",
  "c-migration": "Moving the token tables, 3 of 7 done",
  "c-qa": "Running the checkout tests",
  "c-security": "Getting ready",
  "e-working": "Splitting the billing work into four tasks",
  "e-needs": "Has 2 suggestions for you",
  "e-failed": "Couldn't finish its last check",
  "e-starting": "Getting ready",
  "t-release": "Wants to run make deploy",
};

export function doingOf(instance: DraftInstance) {
  if (instance.pauseReason) return PAUSE_COPY[instance.pauseReason];
  return DOING[instance.id] ?? "Nothing to do right now";
}

/** Oldest first: what the last check-ins came to. */
export type CheckMark = "quiet" | "reported" | "failed";

const CALM: readonly CheckMark[] = [
  "quiet",
  "quiet",
  "reported",
  "quiet",
  "quiet",
  "quiet",
  "reported",
  "quiet",
  "quiet",
  "quiet",
  "quiet",
  "quiet",
];

export function checksOf(instance: DraftInstance): readonly CheckMark[] {
  if (instance.state === "failed") return [...CALM.slice(3), "failed", "failed", "failed"];
  if (instance.state === "needs-you") return [...CALM.slice(1), "reported"];
  return CALM;
}

const RANK: Record<DraftInstance["state"], number> = {
  "needs-you": 0,
  failed: 1,
  working: 2,
  starting: 3,
  idle: 4,
  unknown: 5,
  paused: 6,
};

/** Who wants attention comes first, so a crowded row still leads with what matters. */
export function byUrgency(instances: readonly DraftInstance[]) {
  return [...instances].sort((a, b) => RANK[a.state] - RANK[b.state]);
}

export const RUNTIME_PROVIDERS = [
  { id: "claude", name: "Claude", runtime_provider: "claude", harness: "acp" },
];

export const RUNTIME_MODELS = [
  {
    id: "claude-opus-5-5",
    provider: "claude",
    name: "Claude Opus 5.5",
    featured: true,
    efforts: ["low", "medium", "high"],
    default_effort: "medium",
    availability: "live",
    curated: true,
    supports_tools: true,
  },
] as const;
