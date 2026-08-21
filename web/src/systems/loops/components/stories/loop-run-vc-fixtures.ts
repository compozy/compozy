import type { LoopBriefing, LoopRequest } from "../../types";
import {
  registerDoneScenario,
  registerFailedScenario,
  registerNeedsYouScenario,
  registerRunningScenario,
} from "./loop-run-register-fixtures";
import { STORY_RUN_ID, reviewAndFixRun } from "./loop-run-page-fixture-world";
import {
  asRunStatus,
  makeGeneration as generation,
  makeRosterNode as rosterNode,
  storyAt,
} from "./loop-run-read-builders";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";

/**
 * The visual-contract states a live daemon cannot be held in.
 *
 * Every scenario here is an override of one already in the register world, so a
 * capture still travels the production projection rather than a hand-assembled
 * view model. Nothing invents a state the runtime cannot reach — these are the
 * same reads, paused at a moment that is hard to catch on purpose.
 */

/**
 * A briefing override that keeps the run beside it honest.
 *
 * `LoopRunPageBody` reads `run.status` for the live/terminal split, so a
 * scenario that changes only the briefing stages a contradiction: a "done"
 * verdict over a run the page still paints as running.
 */
function withBriefing(
  scenario: LoopRunStoryScenario,
  overrides: Partial<LoopBriefing>,
  runOverrides: Partial<Parameters<typeof reviewAndFixRun>[0]> = {}
): LoopRunStoryScenario {
  const briefing = { ...scenario.briefing, ...overrides } as LoopBriefing;
  return {
    ...scenario,
    briefing,
    run: reviewAndFixRun({
      ...scenario.run,
      status: asRunStatus(briefing.status, scenario.run.status),
      generation: briefing.progress.round,
      progress: briefing.progress,
      ...runOverrides,
    }),
  };
}

/** VC-02: admitted but not started — the cap is the reason, and it says so. */
export function vcQueuedScenario(): LoopRunStoryScenario {
  return withBriefing(registerRunningScenario(), {
    status: "queued",
    tone: "ok",
    headline: "Waiting to start — one other run of this loop is already going",
    detail: "This loop admits one run at a time.",
    progress: { round: 1, steps_done: 0, steps_total: 4 },
  });
}

/** VC-12: the budget is nearly spent, and the rail warns before it stops. */
export function vcBudgetWarningScenario(): LoopRunStoryScenario {
  const scenario = registerRunningScenario();
  return withBriefing(
    scenario,
    { usage: { tokens: 468_000, cost_usd: 2.34, budget_used_pct: 93.6, duration: "19m40s" } },
    { tokens_used: 468_000, budget_tokens: 500_000 }
  );
}

/** VC-15: the request states its expiry plainly, and never retries itself. */
export function vcRequestExpiryScenario(): LoopRunStoryScenario {
  const scenario = registerNeedsYouScenario();
  // The blocker describes the question; it is not the question. `LoopRunNeedsYouCard`
  // renders the questionnaire from `requests`, so a scenario with a request-shaped
  // blocker and no request staged an expiry notice with nothing expiring under it.
  const request: LoopRequest = {
    loop_run_id: STORY_RUN_ID,
    node_id: "fix_batch",
    item_index: 0,
    generation: 2,
    kind: "question",
    state: "pending",
    agents: "review_fixer",
    prompt: "The two review notes disagree about the retry window. Which one should win?",
    decisions: ["reviewer", "fixer"],
    context: null,
    opened_at: storyAt(3),
    expires_at: storyAt(-4),
  };
  return {
    ...withBriefing(scenario, {
      headline: 'The question on "fix_batch" expires in 4m',
      detail: "Nothing is retried automatically; an expired question simply closes.",
      blockers: [
        {
          kind: "request",
          node_id: "fix_batch",
          item_index: 0,
          waiting_since: storyAt(3),
          expires_at: storyAt(-4),
          unblocker: `compozy loop respond ${STORY_RUN_ID} --node fix_batch`,
        },
      ],
    }),
    requests: [request],
  };
}

/** VC-19: a failed step beside a quarantined one — the two read differently. */
export function vcFailedAndQuarantinedScenario(): LoopRunStoryScenario {
  const scenario = registerFailedScenario();
  return {
    ...scenario,
    rosterNodes: [
      rosterNode("review", "succeeded", { session_id: "ses-77120a3f" }),
      rosterNode("fix_batch", "failed", {
        attempt: 3,
        session_id: "ses-5d871c99",
        started_at: storyAt(4),
        ended_at: storyAt(1),
        attempts: [
          {
            attempt: 1,
            state: "failed",
            disposition: "retried",
            failure_class: "tool_error",
            started_at: storyAt(7),
            ended_at: storyAt(6),
          },
          {
            attempt: 2,
            state: "failed",
            disposition: "retried",
            failure_class: "timeout",
            started_at: storyAt(6),
            ended_at: storyAt(3),
          },
          {
            attempt: 3,
            state: "failed",
            disposition: "settled",
            failure_class: "tool_error",
            started_at: storyAt(4),
            ended_at: storyAt(1),
          },
        ],
      }),
      rosterNode("collect_fixes", "quarantined", { attempt: 4 }),
      rosterNode("write_artifacts", "not_taken"),
    ],
  };
}

/** VC-27: the loop's own strategy cancelled one step; an operator cancelled another. */
export function vcCancelDispositionsScenario(): LoopRunStoryScenario {
  const scenario = registerRunningScenario();
  return {
    ...scenario,
    run: reviewAndFixRun({ status: "canceled" }),
    rosterNodes: [
      rosterNode("review", "succeeded", { session_id: "ses-77120a3f" }),
      rosterNode("fix_batch", "canceled", {
        started_at: storyAt(4),
        ended_at: storyAt(3),
        cancellation: {
          disposition: "strategy",
          cause: "Its siblings already satisfied the gate.",
        },
      }),
      rosterNode("collect_fixes", "canceled", {
        started_at: storyAt(4),
        ended_at: storyAt(3),
        cancellation: {
          disposition: "operator",
          actor_kind: "operator",
          actor_ref: "pedro",
          cause: "Stopped by hand while the run was still going.",
        },
      }),
    ],
  };
}

/** VC-30: rounds with a score beside rounds the loop never scored. */
export function vcGenerationHistoryScenario(): LoopRunStoryScenario {
  const scenario = registerDoneScenario();
  return {
    ...scenario,
    generations: [
      generation(1, {
        verdicts: [
          {
            blocking_issues: [],
            criteria: [],
            gate_id: "quality",
            item_index: 0,
            outcome: "rejected",
            score: 0.42,
          },
        ],
      }),
      generation(2, {
        verdicts: [
          {
            blocking_issues: [],
            criteria: [],
            gate_id: "quality",
            item_index: 0,
            outcome: "approved",
            score: 0.91,
          },
        ],
      }),
      // No verdict at all: a loop that defines no scoring gets no score column.
      generation(3),
    ],
    rosterNodes: [
      rosterNode("review", "succeeded", { generation: 1, usage: { tokens: 24_100 } }),
      rosterNode("fix_batch", "succeeded", { generation: 2, usage: { tokens: 31_800 } }),
      rosterNode("write_artifacts", "succeeded", { generation: 3, usage: { tokens: 8_400 } }),
    ],
  };
}

/** VC-31: the run died mid-round, and the round says so instead of settling. */
export function vcCrashInterruptedScenario(): LoopRunStoryScenario {
  const scenario = vcGenerationHistoryScenario();
  return {
    ...scenario,
    run: reviewAndFixRun({ status: "failed" }),
    rosterNodes: [
      rosterNode("review", "succeeded", { generation: 1, usage: { tokens: 24_100 } }),
      // Started, never ended, on a run that is over: interrupted, not settled.
      rosterNode("fix_batch", "running", {
        generation: 2,
        started_at: storyAt(4),
        usage: { tokens: 12_000 },
      }),
    ],
  };
}

/** VC-32: retention took the session; the panel says so instead of 404ing. */
export function vcPrunedSessionScenario(): LoopRunStoryScenario {
  const scenario = registerDoneScenario();
  return {
    ...scenario,
    rosterNodes: [
      rosterNode("review", "succeeded", {
        session_id: "ses-77120a3f",
        cell_task_id: "task_review",
        started_at: storyAt(15),
        ended_at: storyAt(12),
        attempts: [
          {
            attempt: 1,
            state: "succeeded",
            disposition: "settled",
            started_at: storyAt(15),
            ended_at: storyAt(12),
          },
        ],
      }),
    ],
  };
}

/** The one session `vcPrunedSessionScenario` expects retention to have removed. */
export const VC_PRUNED_SESSION_IDS: ReadonlySet<string> = new Set(["ses-77120a3f"]);
