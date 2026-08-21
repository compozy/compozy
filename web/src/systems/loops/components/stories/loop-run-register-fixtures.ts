import type { LoopBriefing, LoopFanoutRollup, LoopRunRecord, LoopTimelineEntry } from "../../types";
import {
  makeBriefing,
  makeRosterNode as node,
  makeTimelineEntry as entry,
  storyAt,
} from "./loop-run-read-builders";
import {
  STORY_RUN_ID,
  reviewAndFixDefinition,
  reviewAndFixRun,
} from "./loop-run-page-fixture-world";
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

/**
 * One coherent server snapshot per scenario.
 *
 * The run record and the briefing describe the same run, so they cannot be set
 * independently. `LoopRunPageBody` branches on `run.status` for the live/terminal
 * split and for whether the Needs-you section exists at all, so a scenario that
 * moved only `briefing.status` staged a *contradiction*: a briefing saying
 * "done" over a run still rendering as live, or a needs-you headline with no
 * decision card beneath it. Those captures were plausible and false, which is
 * the one thing a visual contract must never be.
 */
interface StoryReadState {
  /** The status both reads agree on. */
  status: LoopRunRecord["status"];
  /** The progress both reads agree on. */
  progress: LoopBriefing["progress"];
}

function readState(
  { status, progress }: StoryReadState,
  briefingOverrides: Partial<LoopBriefing> = {}
): Pick<LoopRunStoryScenario, "run" | "briefing"> {
  return {
    run: reviewAndFixRun({ status, generation: progress.round, progress }),
    briefing: makeBriefing({ status, progress, ...briefingOverrides }),
  };
}

const RUNNING: StoryReadState = {
  status: "running",
  progress: { round: 2, steps_done: 2, steps_total: 4 },
};

function base(overrides: Partial<LoopRunStoryScenario> = {}): LoopRunStoryScenario {
  return {
    ...readState(RUNNING),
    definition: reviewAndFixDefinition,
    frames: [],
    generations: [],
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
    ...readState(
      { status: "needs-approval", progress: { round: 2, steps_done: 2, steps_total: 4 } },
      {
        tone: "needs_you",
        headline: 'The gate "finalize_round" has been waiting 3m for your decision',
        detail: "Nothing else can move until you approve or reject the corrections.",
        blockers: [
          {
            kind: "approval",
            gate_id: "finalize_round",
            waiting_since: storyAt(3),
            unblocker: `compozy loop approve ${STORY_RUN_ID} --gate finalize_round`,
          },
        ],
      }
    ),
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
    ...readState(
      { status: "done", progress: { round: 2, steps_done: 4, steps_total: 4 } },
      {
        headline: "The draft was rewritten and both review notes survived",
        detail: "Two rounds, 18m12s.",
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
    ),
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
    ...readState(
      { status: "failed", progress: { round: 2, steps_done: 1, steps_total: 4 } },
      {
        tone: "failed",
        headline: "The reviewer never came back",
        detail: "Three attempts, all refused by the model. Nothing downstream started.",
        outcome: { status: "failed", cause: "model_refusal", at: storyAt(1) },
      }
    ),
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
            started_at: storyAt(2),
            ended_at: storyAt(1),
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
    ...readState({ status: "running", progress: { round: 2, steps_done: 8, steps_total: 12 } }),
  });
}

/** VC-26: ten attempts stay one row, and the next retry is named. */
export function registerRetryingScenario(): LoopRunStoryScenario {
  return base({
    rosterNodes: [
      node("review", "succeeded"),
      node("fix_batch", "retrying", {
        attempt: 10,
        next_retry_at: storyAt(-2),
        started_at: storyAt(7),
        attempts: Array.from({ length: 10 }, (_unused, index) => ({
          attempt: index + 1,
          state: index === 9 ? "retrying" : "failed",
          disposition: "retried",
          failure_class: index === 8 ? "timeout" : "tool_error",
          started_at: storyAt(7),
          ended_at: storyAt(6),
        })),
      }),
    ],
  });
}

/** VC-28: a run that ended before it reached a single step, said plainly. */
export function registerNoStepsScenario(): LoopRunStoryScenario {
  return base({
    ...readState(
      { status: "no-op", progress: { round: 1, steps_done: 0, steps_total: 0 } },
      {
        headline: "Nothing to do — the event never arrived",
        detail: "The run watched for 24h and settled without executing a step.",
        outcome: { status: "no-op", cause: "no_work", at: storyAt(45) },
      }
    ),
    rosterNodes: [],
    timeline: [],
  });
}

/**
 * The long run VC-10 and E2E-015 are actually about: more than 500 events.
 *
 * Generated rather than written out, because the point is the *shape* — a run
 * whose history does not fit in one page — and 500 hand-written literals would
 * be 500 more things to keep in agreement with each other. Newest first, exactly
 * as the daemon serves it.
 */
export const LONG_STORY_EVENT_COUNT = 620;
/** One page of the durable timeline, matching `TIMELINE_PAGE_LIMIT`. */
export const LONG_STORY_PAGE_SIZE = 50;

export function longStoryTimeline(): LoopTimelineEntry[] {
  const entries: LoopTimelineEntry[] = [];
  for (let index = 0; index < LONG_STORY_EVENT_COUNT; index += 1) {
    const seq = LONG_STORY_EVENT_COUNT - index;
    const round = seq > 400 ? 3 : seq > 200 ? 2 : 1;
    if (seq === 400 || seq === 200) {
      entries.push(
        entry(seq, "generation_started", `round ${round} started`, {
          generation: round,
          at: storyAt(index),
        })
      );
      continue;
    }
    entries.push(
      seq % 7 === 0
        ? entry(seq, "node_succeeded", "step fix batch succeeded", {
            generation: round,
            at: storyAt(index),
          })
        : entry(seq, "node_running", "step fix batch started", {
            generation: round,
            at: storyAt(index),
          })
    );
  }
  // The oldest beat is the fork point, so paging all the way back reaches the
  // one entry US-009.EC-3 is about.
  entries.push(
    entry(1, "run_forked", "forked from round 1 of an earlier run", {
      generation: 1,
      at: storyAt(LONG_STORY_EVENT_COUNT),
    })
  );
  return entries;
}

/** VC-10: a long story whose oldest history is still one click away. */
export function registerLongStoryScenario(): LoopRunStoryScenario {
  return base({ timeline: longStoryTimeline(), timelinePageSize: LONG_STORY_PAGE_SIZE });
}
