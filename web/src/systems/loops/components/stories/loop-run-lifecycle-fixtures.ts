import {
  QUARANTINE_ENTRY,
  control,
  retryingGenerations,
  wait,
} from "./loop-run-lifecycle-payloads";
import { briefingFor } from "./loop-run-read-builders";
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

/**
 * Node-lifecycle run-page scenarios (Spec 1). Each one carries the durable
 * payloads the daemon actually returns — `node_controls[]`, `waits[]`, and the
 * generation outputs — alongside the SSE frames, so the stories exercise the
 * same projection the live page does instead of hand-built view models. The
 * payloads themselves live in `loop-run-lifecycle-payloads`.
 *
 * Every scenario states its own served verdict through `briefingFor`; a parked
 * lane is a blocker the daemon reports, so these strips lead on it rather than
 * reading as a calm run with a quiet note somewhere below.
 *
 * These are the Visual Contract capture targets for VC-R1..VC-R4.
 */

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
  const run = reviewAndFixRun({ tokens_used: 71_000, created_at: minutesAgo(14) });
  return {
    run,
    // A backoff is a blocker nobody can answer, so it inks degraded rather than
    // needs-you: the run is worse off, but there is no decision waiting.
    briefing: briefingFor(run, {
      tone: "degraded",
      headline: "The fix step is waiting to retry after a transport failure",
      detail: "Attempt 2 of the third finding starts in about a minute.",
      blockers: [
        {
          kind: "backoff",
          node_id: "fix_batch",
          item_index: 3,
          waiting_since: minutesAgo(2),
          unblocker: "",
        },
      ],
    }),
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
  const run = reviewAndFixRun({ tokens_used: 66_000, created_at: minutesAgo(26) });
  return {
    run,
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Pedro held one lane; the rest of round 2 is still moving",
      detail: "The third finding waits for the schema. Nothing else is affected.",
    }),
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
  const run = reviewAndFixRun({ tokens_used: 64_000, created_at: minutesAgo(31) });
  return {
    run,
    // A rule parked it, so the sentence names the rule. "Paused" without a
    // reason would read as somebody's choice, which is the wrong story.
    briefing: briefingFor(run, {
      tone: "degraded",
      headline: "A retry-storm rule parked the fix step",
      detail:
        "Five transport failures in ten minutes. The lane stays held until someone clears it.",
    }),
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
  const run = reviewAndFixRun({ tokens_used: 62_000, created_at: minutesAgo(28) });
  return {
    run,
    briefing: briefingFor(run, {
      tone: "degraded",
      headline: "Three steps of round 2 are waiting on something outside the run",
      detail: "A timer, a staging event, and an approval that has already escalated once.",
    }),
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
  const run = reviewAndFixRun({ tokens_used: 78_000, created_at: minutesAgo(31) });
  return {
    run,
    // Quarantine is a decision waiting for a person, so it inks needs-you and
    // the blocker carries the exact command that clears it.
    briefing: briefingFor(run, {
      tone: "needs_you",
      headline: "The fix step is set aside and needs your decision",
      detail: "GitHub rejected the credential twice. The join downstream cannot start without it.",
      blockers: [
        {
          kind: "quarantine",
          node_id: "fix_batch",
          item_index: 3,
          waiting_since: minutesAgo(9),
          unblocker: `compozy loop node requeue --run-id ${run.id} --node fix_batch`,
        },
      ],
    }),
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
  const run = reviewAndFixRun({ tokens_used: 69_000, created_at: minutesAgo(38) });
  return {
    run,
    // A silence flag is evidence, not a verdict: the step is still running and
    // nothing is owed, so the strip reports the quiet without raising an alarm.
    briefing: briefingFor(run, {
      tone: "degraded",
      headline: "The fix step has been quiet for 31 minutes",
      detail: "No output, tool call or heartbeat since then. It has not failed.",
    }),
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

/** VC-R2 — the `canceled` terminal reached by operator cancellation. */
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
  const run = reviewAndFixRun({
    status: "canceled",
    tokens_used: 70_000,
    created_at: minutesAgo(29),
    last_progress_at: minutesAgo(1),
  });
  return {
    run,
    // Who stopped it and when are half the answer each, and the outcome row
    // carries both (US-008.EC-2). What survived is the other half: round 1's
    // artifact is listed, so the page never implies the work was thrown away.
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Pedro stopped this run partway through round 2",
      detail: "The step that had already finished is kept and readable; nothing after it started.",
      outcome: {
        status: "canceled",
        cause: "operator_cancel",
        actor_kind: "user",
        actor_ref: "pedro",
        at: minutesAgo(1),
      },
      artifacts: [
        {
          name: "review-findings.md",
          output: "write_artifacts",
          availability: "available",
          ref: "sha256:6b41d7c2",
        },
      ],
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
  const run = reviewAndFixRun({ tokens_used: 80_000, created_at: minutesAgo(40) });
  return {
    run,
    // Three lanes parked three different ways. The daemon orders blockers
    // quarantine before request before backoff, so the quarantine leads and the
    // headline counts the rest rather than listing them twice.
    briefing: briefingFor(run, {
      tone: "needs_you",
      headline: "Nothing in round 2 is moving: three steps are parked",
      detail: "One set aside for a credential, one held by Pedro, one waiting on a staging event.",
      blockers: [
        {
          kind: "quarantine",
          node_id: "fix_batch",
          item_index: 3,
          waiting_since: minutesAgo(9),
          unblocker: `compozy loop node requeue --run-id ${run.id} --node fix_batch`,
        },
      ],
    }),
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
