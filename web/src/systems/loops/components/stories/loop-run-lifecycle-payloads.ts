import { generationsFor, minutesAgo } from "./loop-run-page-fixture-world";
import type { LoopNodeControl, LoopNodeWait, LoopRunGeneration } from "../../types";

/**
 * The durable node payloads the lifecycle scenarios stage.
 *
 * `node_controls[]`, `waits[]` and the retry columns on a generation output are
 * exactly what `getLoopRun` returns, and they are bulky enough that keeping them
 * beside the scenarios buried the scenarios. They live here so each scenario
 * reads as the state it stages rather than as the payload it carries.
 */

export function control(
  overrides: Partial<LoopNodeControl> & { node_id: string }
): LoopNodeControl {
  return {
    loop_run_id: "r-7c4e19",
    paused: false,
    quarantined: false,
    death_resume_streak: 0,
    revision: 1,
    updated_at: minutesAgo(2),
    ...overrides,
  };
}

export function wait(overrides: Partial<LoopNodeWait> & { node_id: string }): LoopNodeWait {
  return {
    loop_run_id: "r-7c4e19",
    generation: 2,
    item_index: 0,
    kind: "timer",
    escalation_cursor: 0,
    claim_state: "waiting",
    admission_failures: 0,
    issued_epoch: 4,
    created_at: minutesAgo(18),
    age_seconds: 18 * 60,
    ...overrides,
  };
}

/** Adds the retry columns the daemon writes on a node inside its backoff. */
export function retryingGenerations(): LoopRunGeneration[] {
  const generations = generationsFor("failed");
  const outputs = generations[0].outputs.map(output =>
    output.node_id === "fix_batch" && output.item_index === 3
      ? {
          ...output,
          attempt: 2,
          next_attempt_at: minutesAgo(-1),
          failure_class: "transport",
          disposition: "retried",
        }
      : output
  );
  return [{ ...generations[0], outputs }];
}

/** The two-episode repair record behind the quarantined and parked scenarios. */
export const QUARANTINE_ENTRY = {
  node_id: "fix_batch",
  input_ref: ".compozy/tasks/loops-paper/task_03.md",
  target: "toolcall:github-mcp",
  episodes: [
    {
      generation: 1,
      quarantined_at: minutesAgo(21),
      attempts: [
        {
          attempt: 1,
          failure_class: "transport",
          cause: "Push to GitHub timed out",
          hint: "The fix landed locally but the push never completed.",
          disposition: "retried",
          ended_at: minutesAgo(24),
        },
        {
          attempt: 2,
          failure_class: "attempt_timeout",
          cause: "Attempt ran past its 5-minute deadline",
          hint: "The retry hung on the same push and was cut at the node's own time limit.",
          disposition: "retried",
          ended_at: minutesAgo(22),
        },
        {
          attempt: 3,
          failure_class: "payload_declared",
          cause: "GitHub rejected the credential",
          hint: "Authentication failed — this class never auto-retries, so the failure routed to on_failed.",
          disposition: "routed",
          ended_at: minutesAgo(21),
        },
      ],
    },
    {
      generation: 2,
      quarantined_at: minutesAgo(9),
      attempts: [
        {
          attempt: 4,
          failure_class: "payload_declared",
          cause: "Same rejection — set aside again",
          hint: "Rotate the github-mcp credential in Vault, then requeue this lane.",
          disposition: "quarantined",
          ended_at: minutesAgo(9),
        },
      ],
    },
  ],
  requeues: [
    {
      actor_kind: "user",
      actor_id: "pedro",
      reason: "rotated the token",
      requested_at: minutesAgo(12),
      generation: 2,
    },
  ],
};
