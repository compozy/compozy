import type { LoopRequest, LoopRunRecord } from "../../types";
import {
  registerDoneScenario,
  registerFailedScenario,
  registerNeedsYouScenario,
  registerRunningScenario,
} from "./loop-run-register-fixtures";
import { STORY_RUN_ID, reviewAndFixRun } from "./loop-run-page-fixture-world";
import {
  type StoryVerdict,
  briefingFor,
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
 * Restage a scenario's run, and derive its briefing from the result.
 *
 * The previous helper went the other way — it merged briefing overrides, cast
 * the result, and rebuilt the run from the briefing — and every scenario that
 * bypassed it by assigning `run:` directly kept the briefing it inherited. That
 * is how VC-27 came to photograph a calm "Reviewing the second draft" over a
 * canceled run and VC-31 a "Done" verdict over a failed one. Deriving forward
 * from the run makes the disagreement unrepresentable rather than discouraged.
 */
function restage(
  scenario: LoopRunStoryScenario,
  runOverrides: Partial<LoopRunRecord>,
  verdict: StoryVerdict
): LoopRunStoryScenario {
  const run = reviewAndFixRun({ ...scenario.run, ...runOverrides });
  return { ...scenario, run, briefing: briefingFor(run, verdict) };
}

/** The calm running verdict the register world serves, restated where inherited. */
const RUNNING_VERDICT: StoryVerdict = {
  tone: "ok",
  headline: "Reviewing the second draft",
  detail: "Nothing needs you. Two of four steps are done in round 2.",
  usage: { cost_usd: 0.31, duration: "9m40s" },
};

/** VC-02: admitted but not started — the cap is the reason, and it says so. */
export function vcQueuedScenario(): LoopRunStoryScenario {
  return restage(
    registerRunningScenario(),
    {
      status: "queued",
      generation: 1,
      progress: { round: 1, steps_done: 0, steps_total: 4 },
      // Admitted and not started: nothing has been spent yet.
      tokens_used: 0,
    },
    {
      tone: "ok",
      headline: "Waiting to start — one other run of this loop is already going",
      detail: "This loop admits one run at a time.",
    }
  );
}

/** VC-12: the budget is nearly spent, and the rail warns before it stops. */
export function vcBudgetWarningScenario(): LoopRunStoryScenario {
  // The percentage is arithmetic on the record, not a second opinion about it:
  // 468k of 500k is the 93.6% the rail warns on.
  return restage(
    registerRunningScenario(),
    { tokens_used: 468_000, budget_tokens: 500_000 },
    { ...RUNNING_VERDICT, usage: { cost_usd: 2.34, duration: "19m40s" } }
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
    ...restage(
      scenario,
      {},
      {
        tone: "needs_you",
        headline: "The question on the fix batch step expires in 4m",
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
      }
    ),
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
  const scenario = restage(
    registerRunningScenario(),
    { status: "canceled" },
    {
      // `briefing.go` tones every non-failed terminal status `ok`; a cancellation
      // is a decision, not a fault, and the neutral outcome pill carries it.
      tone: "ok",
      headline: "The run was stopped before it finished",
      detail: "Two of four steps had settled in round 2.",
      outcome: {
        status: "canceled",
        cause: "operator_cancel",
        at: storyAt(3),
        actor_kind: "operator",
        actor_ref: "pedro",
      },
      usage: { cost_usd: 0.31, duration: "9m40s" },
    }
  );
  return {
    ...scenario,
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
  // Three generations means the run reached round 3; leaving the record at
  // round 2 put "Rounds 2 / 3" in the rail above a history listing a round 3.
  const scenario = restage(
    registerDoneScenario(),
    { generation: 3, progress: { round: 3, steps_done: 3, steps_total: 3 } },
    {
      tone: "ok",
      headline: "The draft was rewritten and both review notes survived",
      detail: "Three rounds, 18m12s.",
      usage: { cost_usd: 0.87, duration: "18m12s" },
      outcome: { status: "done", cause: "verified", at: storyAt(47) },
      artifacts: [
        {
          name: "post-final.md",
          output: "write_artifacts",
          availability: "available",
          ref: "sha256:2f81c4a9",
        },
      ],
    }
  );
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
      // Round 3 is the round the DAG draws, and this scenario is also VC-18's
      // terminal graph. Without a row per node for that round the graph read
      // every unlisted step as "Reachable. Nothing has reached it yet." — a
      // finished run drawn as though it were still on its way.
      rosterNode("review", "succeeded", { generation: 3 }),
      rosterNode("has_issues", "succeeded", { generation: 3 }),
      // The round found no issues, so the fix branch was provably declined —
      // durable route evidence, not a step still waiting its turn.
      rosterNode("fix_batches", "not_taken", { generation: 3 }),
      rosterNode("fix_batch", "not_taken", { generation: 3 }),
      rosterNode("collect_fixes", "not_taken", { generation: 3 }),
      rosterNode("finalize_round", "succeeded", { generation: 3 }),
    ],
  };
}

/** VC-31: the run died mid-round, and the round says so instead of settling. */
export function vcCrashInterruptedScenario(): LoopRunStoryScenario {
  const history = vcGenerationHistoryScenario();
  // The daemon restarted inside round 2, so the run never reached round 3 and
  // its verdict is `failed` — the previous fixture kept the done briefing and
  // captured a "Done" strip over a failed run.
  const scenario = restage(
    history,
    { status: "failed", generation: 2, progress: { round: 2, steps_done: 1, steps_total: 3 } },
    {
      tone: "failed",
      headline: "The daemon restarted while round 2 was still running",
      detail: "The step that was in flight never wrote its end, so its duration is unknown.",
      outcome: { status: "failed", cause: "daemon_restart", at: storyAt(3) },
    }
  );
  return {
    ...scenario,
    generations: [
      // Round 1 settled and keeps its verdict.
      history.generations[0] as (typeof history.generations)[number],
      // Round 2 has no verdict, because the gate never got to write one. Keeping
      // the history's `approved` here printed an `accepted` pill beside the words
      // "interrupted before it finished" — the exact back-filled guess the
      // reference's own spec note forbids. The step that did settle keeps its
      // result; the one in flight recorded nothing.
      generation(2, { outputs: [{ node_id: "review", status: "succeeded", generation: 2 }] }),
    ],
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
