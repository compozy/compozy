import { RELEASE, INSTANCES, ORCHESTRATOR, SCOUT, TRIAGE, type DraftInstance } from "./fixtures";
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
