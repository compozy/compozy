import type { LoopNodeLifecycle } from "../lib/loop-node-lifecycle";

/** Default-valid lifecycle row for loop presentation tests. */
export function loopNodeLifecycleFixture(
  overrides: Partial<LoopNodeLifecycle> = {}
): LoopNodeLifecycle {
  return {
    nodeId: "task_03",
    label: "task 03",
    state: null,
    parked: false,
    paused: false,
    pauseProvenance: null,
    quarantined: false,
    quarantinedAt: null,
    quarantineEntry: null,
    attentionFlag: "",
    attentionReason: "",
    attentionProducerNodeId: "",
    cancelState: "",
    cancelProvenance: null,
    lastEvidenceAt: null,
    deathResumeStreak: 0,
    revision: 1,
    waits: [],
    attempt: null,
    nextAttemptAt: null,
    failureClass: "",
    disposition: "",
    outputStatus: "running",
    generation: 2,
    itemIndex: null,
    sessionId: null,
    ...overrides,
  };
}
