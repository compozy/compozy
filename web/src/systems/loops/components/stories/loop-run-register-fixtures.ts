import type {
  LoopBriefing,
  LoopFanoutRollup,
  LoopRosterNode,
  LoopTimelineEntry,
} from "../../types";
import { reviewAndFixDefinition, reviewAndFixRun } from "./loop-run-page-fixture-world";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";

/**
 * Register-bearing scenarios: the three run reads, staged.
 *
 * These exist so the visual-contract capture has real targets for states a live
 * daemon cannot be held in — a pruned artifact, a ten-way fan-out, a route the
 * run declined. They stage the *reads*, not the rendering, so a captured story
 * still travels the production projection (`projectLoopRunRegisters`) rather
 * than a hand-assembled view model that could drift from it.
 *
 * They reuse the existing `review-and-fix` world. No third loop is minted.
 */

const RUN_ID = "r-7c4e19";

function node(
  nodeId: string,
  state: string,
  overrides: Partial<LoopRosterNode> = {}
): LoopRosterNode {
  return {
    generation: 2,
    node_id: nodeId,
    item_index: 0,
    state,
    attempt: 1,
    attempts: [],
    ...overrides,
  } as LoopRosterNode;
}

function entry(
  seq: number,
  kind: string,
  title: string,
  overrides: Partial<LoopTimelineEntry> = {}
): LoopTimelineEntry {
  return {
    seq,
    kind,
    title,
    at: "2026-08-19T18:43:38Z",
    generation: 2,
    ...overrides,
  } as LoopTimelineEntry;
}

function briefing(overrides: Partial<LoopBriefing> = {}): LoopBriefing {
  return {
    run_id: RUN_ID,
    status: "running",
    tone: "ok",
    headline: "Reviewing the second draft",
    detail: "Nothing needs you. Two of four steps are done in round 2.",
    blockers: [],
    artifacts: [],
    progress: { round: 2, steps_done: 2, steps_total: 4 },
    usage: { tokens: 82_400, cost_usd: 0.31, budget_used_pct: 12, duration_ms: 580_000 },
    ...overrides,
  } as LoopBriefing;
}

function base(overrides: Partial<LoopRunStoryScenario> = {}): LoopRunStoryScenario {
  return {
    run: reviewAndFixRun(),
    definition: reviewAndFixDefinition,
    frames: [],
    generations: [],
    briefing: briefing(),
    rosterNodes: [
      node("review", "succeeded", { session_id: "ses-77120a3f", cell_task_id: "task_review" }),
      node("fix_batch", "running", { session_id: "ses-c3f00e42" }),
      node("collect_fixes", "pending"),
      node("write_artifacts", "pending"),
    ],
    rosterRollups: [],
    timeline: [
      entry(90, "node_running", "step fix_batch started"),
      entry(84, "node_succeeded", "step review succeeded"),
      entry(80, "generation_started", "round 2 started"),
    ],
    ...overrides,
  };
}

/** VC-01: the calm running read — nothing needs a person. */
export function registerRunningScenario(): LoopRunStoryScenario {
  return base();
}

/** VC-13/14: a gate holding the run, with the decision card leading. */
export function registerNeedsYouScenario(): LoopRunStoryScenario {
  return base({
    briefing: briefing({
      status: "needs-approval",
      tone: "needs_you",
      headline: 'The gate "finalize_round" has been waiting 3m for your decision',
      detail: "Nothing else can move until you approve or reject the corrections.",
      blockers: [
        {
          kind: "approval",
          gate_id: "finalize_round",
          waiting_since: "2026-08-19T18:41:00Z",
          unblocker: `compozy loop approve ${RUN_ID} --gate finalize_round`,
        },
      ],
    }),
    rosterNodes: [
      node("review", "succeeded", { session_id: "ses-77120a3f" }),
      node("fix_batch", "succeeded", { attempt: 2, session_id: "ses-5d871c99" }),
      node("finalize_round", "control_pending"),
      node("write_artifacts", "pending"),
    ],
  });
}

/** VC-04: a finished run leading with its outcome and what it produced. */
export function registerDoneScenario(): LoopRunStoryScenario {
  return base({
    briefing: briefing({
      status: "done",
      headline: "The draft was rewritten and both review notes survived",
      detail: "Two rounds, 18m12s.",
      outcome: { status: "done", cause: "verified", at: "2026-08-19T17:58:12Z" },
      artifacts: [
        {
          name: "post-final.md",
          output: "write_artifacts",
          availability: "available",
          ref: "blob-2f81",
        },
      ],
      progress: { round: 2, steps_done: 4, steps_total: 4 },
    }),
    rosterNodes: [
      node("review", "succeeded"),
      node("fix_batch", "succeeded"),
      node("collect_fixes", "succeeded"),
      node("write_artifacts", "succeeded"),
    ],
  });
}

/** VC-09: retention removed the bytes; the name and the fact survive. */
export function registerPrunedArtifactScenario(): LoopRunStoryScenario {
  const done = registerDoneScenario();
  return {
    ...done,
    briefing: {
      ...done.briefing!,
      artifacts: [{ name: "post-final.md", output: "write_artifacts", availability: "pruned" }],
    } as LoopBriefing,
  };
}

/** VC-05: a failure that stays visible with everything collapsed. */
export function registerFailedScenario(): LoopRunStoryScenario {
  return base({
    briefing: briefing({
      status: "failed",
      tone: "failed",
      headline: "The reviewer never came back",
      detail: "Three attempts, all refused by the model. Nothing downstream started.",
      outcome: { status: "failed", cause: "model_refusal", at: "2026-08-19T18:44:00Z" },
      progress: { round: 2, steps_done: 1, steps_total: 4 },
    }),
    rosterNodes: [
      node("review", "succeeded"),
      node("fix_batch", "failed", {
        attempt: 3,
        attempts: [
          {
            attempt: 3,
            state: "failed",
            disposition: "exhausted",
            failure_class: "model_refusal",
            started_at: "2026-08-19T18:43:00Z",
            ended_at: "2026-08-19T18:44:00Z",
          },
        ],
      }),
      node("collect_fixes", "pending"),
    ],
  });
}

/** VC-20: `pending` and `not_taken` side by side — the distinction SI-14 requires. */
export function registerRoutedScenario(): LoopRunStoryScenario {
  return base({
    rosterNodes: [
      node("review", "succeeded"),
      node("has_issues", "succeeded"),
      // Durable route evidence: the run provably went elsewhere.
      node("write_artifacts", "not_taken"),
      // Reachable, simply not reached yet.
      node("collect_fixes", "pending"),
    ],
  });
}

/** VC-21: ten workers stay one entity carrying a rollup. */
export function registerWideFanOutScenario(): LoopRunStoryScenario {
  const rollup: LoopFanoutRollup = {
    generation: 2,
    node_id: "fix_batches",
    done: 7,
    total: 10,
    failed: 1,
  };
  return base({
    rosterNodes: [
      node("review", "succeeded"),
      ...Array.from({ length: 10 }, (_unused, index) =>
        node("fix_batch", index < 7 ? "succeeded" : index === 7 ? "failed" : "running", {
          item_index: index,
        })
      ),
      node("collect_fixes", "pending"),
    ],
    rosterRollups: [rollup],
    briefing: briefing({ progress: { round: 2, steps_done: 8, steps_total: 12 } }),
  });
}

/** VC-26: ten attempts stay one row, and the next retry is named. */
export function registerRetryingScenario(): LoopRunStoryScenario {
  return base({
    rosterNodes: [
      node("review", "succeeded"),
      node("fix_batch", "retrying", {
        attempt: 10,
        next_retry_at: "2026-08-19T18:47:00Z",
        started_at: "2026-08-19T18:38:00Z",
        attempts: Array.from({ length: 10 }, (_unused, index) => ({
          attempt: index + 1,
          state: index === 9 ? "retrying" : "failed",
          disposition: "retried",
          failure_class: index === 8 ? "timeout" : "tool_error",
          started_at: "2026-08-19T18:38:00Z",
          ended_at: "2026-08-19T18:39:00Z",
        })),
      }),
    ],
  });
}

/** VC-28: a run that ended before it reached a single step, said plainly. */
export function registerNoStepsScenario(): LoopRunStoryScenario {
  return base({
    briefing: briefing({
      status: "no-op",
      headline: "Nothing to do — the event never arrived",
      detail: "The run watched for 24h and settled without executing a step.",
      outcome: { status: "no-op", cause: "no_work", at: "2026-08-19T18:00:00Z" },
      progress: { round: 1, steps_done: 0, steps_total: 0 },
    }),
    rosterNodes: [],
    timeline: [],
  });
}

/** VC-10: a long story whose oldest history is still one click away. */
export function registerLongStoryScenario(): LoopRunStoryScenario {
  return base({
    timeline: [
      // A coalesced chatter run: the daemon folded 142 ticks into one beat.
      entry(220, "token_tick", "progress heartbeats", { first_seq: 79 }),
      entry(78, "node_succeeded", "step review succeeded"),
      entry(40, "run_forked", "forked from round 1 of an earlier run"),
      entry(1, "generation_started", "round 1 started", { generation: 1 }),
    ],
  });
}
