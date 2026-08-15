import type { LoopRunStoryScenario } from "./loop-run-scenario-types";
import {
  createFrameFactory,
  generationsFor,
  minutesAgo,
  nodePayload,
  reviewAndFixDefinition,
  reviewAndFixRun,
  roundOneFrames,
} from "./loop-run-page-fixture-world";
import type { LoopNodeControl, LoopNodeWait, LoopRunGeneration } from "../../types";

/**
 * Node-lifecycle run-page scenarios (Spec 1). Each one carries the durable
 * payloads the daemon actually returns — `node_controls[]`, `waits[]`, and the
 * generation outputs — alongside the SSE frames, so the stories exercise the
 * same projection the live page does instead of hand-built view models.
 *
 * These are the Visual Contract capture targets for VC-R1..VC-R4.
 */

function control(overrides: Partial<LoopNodeControl> & { node_id: string }): LoopNodeControl {
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

function wait(overrides: Partial<LoopNodeWait> & { node_id: string }): LoopNodeWait {
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
function retryingGenerations(): LoopRunGeneration[] {
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

/** VC-R1 — the canonical live mid-retry timeline. */
export function retryingScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 9, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_revise",
      reattempt_strategy: "failed_only",
    }),
    frame("node_running", 8, nodePayload("fix_batch", 2, { item_index: 3 })),
    frame(
      "node_failed",
      2,
      nodePayload("fix_batch", 2, { item_index: 3, attempt: 1, disposition: "retried" })
    ),
    frame(
      "node_retry_scheduled",
      1,
      nodePayload("fix_batch", 2, {
        item_index: 3,
        attempt: 2,
        next_attempt_at: minutesAgo(-1),
        failure_class: "transport",
        issued_epoch: 4,
      })
    ),
  ];
  return {
    run: reviewAndFixRun({ tokens_used: 71_000, created_at: minutesAgo(14) }),
    definition: reviewAndFixDefinition,
    frames,
    generations: retryingGenerations(),
  };
}

/** VC-R2 — a lane paused by an operator while the rest of the run works. */
export function pausedNodeScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 9, { generation: 2, parent_generation: 1, origin: "gate_revise" }),
    frame(
      "node_paused",
      6,
      nodePayload("fix_batch", 2, {
        item_index: 3,
        actor_kind: "user",
        actor_id: "pedro",
        reason: "hold this one until the schema lands",
        mode: "drain",
      })
    ),
  ];
  return {
    run: reviewAndFixRun({ tokens_used: 66_000, created_at: minutesAgo(26) }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("running"),
    nodeControls: [
      control({
        node_id: "fix_batch",
        paused: true,
        revision: 2,
        pause_provenance: {
          actor_kind: "user",
          actor_id: "pedro",
          reason: "hold this one until the schema lands",
          requested_at: minutesAgo(6),
        },
      }),
    ],
  };
}

/** VC-R2 — the same park, but opened by a rule instead of an operator. */
export function pausedByRuleScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 9, { generation: 2, parent_generation: 1, origin: "gate_revise" }),
    frame(
      "node_paused",
      6,
      nodePayload("fix_batch", 2, {
        item_index: 3,
        actor_kind: "system",
        rule_id: "retry_storm",
        reason: "5 transport failures in 10 minutes",
        mode: "drain",
      })
    ),
  ];
  return {
    run: reviewAndFixRun({ tokens_used: 64_000, created_at: minutesAgo(31) }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("running"),
    nodeControls: [
      control({
        node_id: "fix_batch",
        paused: true,
        revision: 2,
        pause_provenance: {
          actor_kind: "system",
          actor_id: "autopause",
          rule_id: "retry_storm",
          reason: "5 transport failures in 10 minutes",
          requested_at: minutesAgo(4),
        },
      }),
    ],
  };
}

/** VC-R2 — three open waits: a timer, an event, and an escalating approval. */
export function waitingScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 9, { generation: 2, parent_generation: 1, origin: "gate_revise" }),
    frame(
      "node_wait_started",
      6,
      nodePayload("fix_batch", 2, {
        item_index: 3,
        wait_kind: "event",
        resume_at: null,
        issued_epoch: 4,
      })
    ),
  ];
  return {
    run: reviewAndFixRun({ tokens_used: 62_000, created_at: minutesAgo(28) }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("reused"),
    waits: [
      wait({
        node_id: "collect_fixes",
        kind: "timer",
        resume_at: minutesAgo(-24),
        created_at: minutesAgo(6),
        age_seconds: 360,
      }),
      wait({
        node_id: "fix_batch",
        item_index: 3,
        kind: "event",
        age_seconds: 18 * 60,
        expect: { env: "staging" },
      }),
      wait({
        node_id: "finalize_round",
        kind: "approval_escalation",
        claim_state: "intervention_required",
        escalation_cursor: 1,
        created_at: minutesAgo(22),
        age_seconds: 22 * 60,
        next_escalation_at: minutesAgo(-8),
      }),
    ],
  };
}

const QUARANTINE_ENTRY = {
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

/** VC-R2 + VC-R4 — a set-aside lane with a two-episode repair record. */
export function quarantinedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 9, { generation: 2, parent_generation: 1, origin: "gate_revise" }),
    frame(
      "node_quarantined",
      4,
      nodePayload("fix_batch", 2, {
        item_index: 3,
        attempt: 4,
        disposition: "quarantined",
        failure: { cause: "GitHub rejected the credential twice in a row." },
      })
    ),
    frame(
      "node_attention_flagged",
      2,
      nodePayload("collect_fixes", 2, {
        attention_flag: "dependency_quarantined",
        reason: "blocked on fix_batch",
      })
    ),
  ];
  return {
    run: reviewAndFixRun({ tokens_used: 78_000, created_at: minutesAgo(31) }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("failed"),
    nodeControls: [
      control({
        node_id: "fix_batch",
        quarantined: true,
        quarantined_at: minutesAgo(9),
        revision: 4,
        quarantine_entry: QUARANTINE_ENTRY,
      }),
      control({
        node_id: "collect_fixes",
        attention_flag: "dependency_quarantined",
        attention_reason: "The join needs the output of fix_batch, which is quarantined.",
        revision: 2,
      }),
    ],
  };
}

/** VC-R2 — a lane that has gone quiet, flagged as evidence only. */
export function attentionScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 9, { generation: 2, parent_generation: 1, origin: "gate_revise" }),
    frame("node_running", 8, nodePayload("fix_batch", 2, { item_index: 3 })),
    frame("node_attention_flagged", 3, nodePayload("fix_batch", 2, { reason: "silence" })),
  ];
  return {
    run: reviewAndFixRun({ tokens_used: 69_000, created_at: minutesAgo(38) }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("running"),
    nodeControls: [
      control({
        node_id: "fix_batch",
        attention_flag: "silence",
        attention_reason: "No output, tool call, or heartbeat for 31 minutes.",
        last_evidence_at: minutesAgo(31),
        revision: 2,
      }),
    ],
  };
}

/** VC-R2 — the `canceled` terminal reached by a cooperative cancel. */
export function canceledScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 9, { generation: 2, parent_generation: 1, origin: "gate_revise" }),
    frame(
      "node_canceled",
      3,
      nodePayload("fix_batch", 2, { item_index: 3, actor_kind: "user", actor_id: "pedro" })
    ),
    frame("status_changed", 1, {
      from: "running",
      to: "canceled",
      status: "canceled",
      cause: "operator_cancel",
    }),
  ];
  return {
    run: reviewAndFixRun({
      status: "canceled",
      tokens_used: 70_000,
      created_at: minutesAgo(29),
      last_progress_at: minutesAgo(1),
    }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("canceled", "succeeded"),
  };
}

/** S8 — mixed parked lanes: one paused, one waiting, one quarantined. */
export function parkedProgressScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 9, { generation: 2, parent_generation: 1, origin: "gate_revise" }),
    frame(
      "node_paused",
      6,
      nodePayload("write_artifacts", 2, {
        actor_kind: "user",
        actor_id: "pedro",
        reason: "hold the write until review lands",
        mode: "drain",
      })
    ),
    frame(
      "node_wait_started",
      5,
      nodePayload("collect_fixes", 2, { wait_kind: "event", resume_at: null, issued_epoch: 4 })
    ),
    frame(
      "node_quarantined",
      4,
      nodePayload("fix_batch", 2, {
        item_index: 3,
        attempt: 4,
        disposition: "quarantined",
        failure: { cause: "GitHub rejected the credential twice in a row." },
      })
    ),
  ];
  return {
    run: reviewAndFixRun({ tokens_used: 80_000, created_at: minutesAgo(40) }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("failed"),
    nodeControls: [
      control({
        node_id: "write_artifacts",
        paused: true,
        revision: 2,
        pause_provenance: {
          actor_kind: "user",
          actor_id: "pedro",
          reason: "hold the write until review lands",
          requested_at: minutesAgo(8),
        },
      }),
      control({
        node_id: "fix_batch",
        quarantined: true,
        quarantined_at: minutesAgo(9),
        revision: 4,
        quarantine_entry: QUARANTINE_ENTRY,
      }),
    ],
    waits: [wait({ node_id: "collect_fixes", kind: "event", age_seconds: 7 * 60 })],
  };
}

/** VC-R2 — the same terminal reached by kill; only the `cause` differs. */
export function killedScenario(): LoopRunStoryScenario {
  const scenario = canceledScenario();
  const frame = createFrameFactory();
  return {
    ...scenario,
    frames: [
      ...scenario.frames.slice(0, -1),
      frame("status_changed", 1, {
        from: "running",
        to: "canceled",
        status: "canceled",
        cause: "operator_kill",
      }),
    ],
  };
}
