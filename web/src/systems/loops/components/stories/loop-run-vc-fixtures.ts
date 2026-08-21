import type { LoopBriefing, LoopRosterNode, LoopRunGeneration } from "../../types";
import {
  registerDoneScenario,
  registerFailedScenario,
  registerNeedsYouScenario,
  registerRunningScenario,
} from "./loop-run-register-fixtures";
import { reviewAndFixRun } from "./loop-run-page-fixture-world";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";

/**
 * The visual-contract states a live daemon cannot be held in.
 *
 * Every scenario here is an override of one already in the register world, so a
 * capture still travels the production projection rather than a hand-assembled
 * view model. Nothing invents a state the runtime cannot reach — these are the
 * same reads, paused at a moment that is hard to catch on purpose.
 */

const RUN_ID = "r-7c4e19";

function withBriefing(
  scenario: LoopRunStoryScenario,
  overrides: Partial<LoopBriefing>
): LoopRunStoryScenario {
  return { ...scenario, briefing: { ...scenario.briefing, ...overrides } as LoopBriefing };
}

function rosterNode(
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

function generation(round: number, overrides: Partial<LoopRunGeneration> = {}): LoopRunGeneration {
  return {
    generation: round,
    origin: round === 1 ? "initial" : "gate_revise",
    parent_generation: round === 1 ? 0 : round - 1,
    outputs: [],
    route_causes: [],
    verdicts: [],
    ...overrides,
  } as LoopRunGeneration;
}

/** VC-02: admitted but not started — the cap is the reason, and it says so. */
export function vcQueuedScenario(): LoopRunStoryScenario {
  return withBriefing(
    { ...registerRunningScenario(), run: reviewAndFixRun({ status: "queued" }) },
    {
      status: "queued",
      tone: "ok",
      headline: "Waiting to start — one other run of this loop is already going",
      detail: "This loop admits one run at a time.",
      progress: { round: 1, steps_done: 0, steps_total: 4 },
    }
  );
}

/** VC-12: the budget is nearly spent, and the rail warns before it stops. */
export function vcBudgetWarningScenario(): LoopRunStoryScenario {
  const scenario = registerRunningScenario();
  return withBriefing(
    { ...scenario, run: reviewAndFixRun({ tokens_used: 468_000, budget_tokens: 500_000 }) },
    {
      usage: { tokens: 468_000, cost_usd: 2.34, budget_used_pct: 93.6, duration_ms: 1_180_000 },
    }
  );
}

/** VC-15: the request states its expiry plainly, and never retries itself. */
export function vcRequestExpiryScenario(): LoopRunStoryScenario {
  const scenario = registerNeedsYouScenario();
  return withBriefing(scenario, {
    headline: 'The question on "fix_batch" expires in 4m',
    detail: "Nothing is retried automatically; an expired question simply closes.",
    blockers: [
      {
        kind: "request",
        node_id: "fix_batch",
        item_index: 0,
        waiting_since: "2026-08-19T18:41:00Z",
        expires_at: "2026-08-19T18:49:00Z",
        unblocker: `compozy loop respond ${RUN_ID} --node fix_batch`,
      },
    ],
  });
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
        started_at: "2026-08-19T18:41:07Z",
        ended_at: "2026-08-19T18:43:38Z",
        attempts: [
          {
            attempt: 1,
            state: "failed",
            disposition: "retried",
            failure_class: "tool_error",
            started_at: "2026-08-19T18:38:00Z",
            ended_at: "2026-08-19T18:39:10Z",
          },
          {
            attempt: 2,
            state: "failed",
            disposition: "retried",
            failure_class: "timeout",
            started_at: "2026-08-19T18:39:20Z",
            ended_at: "2026-08-19T18:41:00Z",
          },
          {
            attempt: 3,
            state: "failed",
            disposition: "settled",
            failure_class: "tool_error",
            started_at: "2026-08-19T18:41:07Z",
            ended_at: "2026-08-19T18:43:38Z",
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
        started_at: "2026-08-19T18:41:07Z",
        ended_at: "2026-08-19T18:42:00Z",
        cancellation: {
          disposition: "strategy",
          cause: "Its siblings already satisfied the gate.",
        },
      }),
      rosterNode("collect_fixes", "canceled", {
        started_at: "2026-08-19T18:41:07Z",
        ended_at: "2026-08-19T18:42:10Z",
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
      } as Partial<LoopRunGeneration>),
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
      } as Partial<LoopRunGeneration>),
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
        started_at: "2026-08-19T18:41:07Z",
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
        started_at: "2026-08-19T18:30:00Z",
        ended_at: "2026-08-19T18:33:00Z",
        attempts: [
          {
            attempt: 1,
            state: "succeeded",
            disposition: "settled",
            started_at: "2026-08-19T18:30:00Z",
            ended_at: "2026-08-19T18:33:00Z",
          },
        ],
      }),
    ],
  };
}

/** The one session `vcPrunedSessionScenario` expects retention to have removed. */
export const VC_PRUNED_SESSION_IDS: ReadonlySet<string> = new Set(["ses-77120a3f"]);
