import type {
  LoopBriefing,
  LoopFanoutRollup,
  LoopRunArtifact,
  LoopRunRecord,
  LoopTimelineEntry,
} from "../../types";
import {
  type StoryVerdict,
  briefingFor,
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
  /** Spend the run record carries, when a scenario is about the spend. */
  spend?: Pick<LoopRunRecord, "tokens_used" | "budget_tokens">;
}

/**
 * The calm verdict the running world serves when nothing needs a person.
 *
 * `cost_usd` and `duration` are stated because the daemon computes them and the
 * run record cannot; everything else about the spend is derived from the record.
 */
const RUNNING_VERDICT: StoryVerdict = {
  tone: "ok",
  headline: "Reviewing the second draft",
  detail: "Nothing needs you. Two of four steps are done in round 2.",
  usage: { cost_usd: 0.31, duration: "9m40s" },
};

/**
 * One run record, one briefing derived from it.
 *
 * The briefing used to be built independently and then re-synchronised on
 * `status` and `progress` alone, which left the spend free to disagree: every
 * register capture showed 82.4k tokens over a run recording 68k. Deriving the
 * briefing from the record removes the disagreement rather than patching it.
 */
function readState(
  { status, progress, spend }: StoryReadState,
  verdict: StoryVerdict = RUNNING_VERDICT
): Pick<LoopRunStoryScenario, "run" | "briefing"> {
  const run = reviewAndFixRun({ status, generation: progress.round, progress, ...spend });
  return { run, briefing: briefingFor(run, verdict) };
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
    // Timestamps and usage are not decoration: a settled node always carries
    // them on the wire, and without them the roster printed "not started"
    // beside a succeeded step and an empty spend column.
    rosterNodes: [
      node("review", "succeeded", {
        session_id: "ses-77120a3f",
        cell_task_id: "task_review",
        started_at: storyAt(14),
        ended_at: storyAt(10),
        usage: { tokens: 31_200 },
      }),
      node("fix_batch", "running", {
        session_id: "ses-c3f00e42",
        started_at: storyAt(3),
        usage: { tokens: 12_400 },
      }),
      node("collect_fixes", "pending"),
      node("write_artifacts", "pending"),
    ],
    rosterRollups: [],
    // Titles are the daemon's own, verbatim: `timelineTitle` in
    // `internal/loop/timeline.go` writes "Step <node_id> <state>" and
    // "Round <n> started". A prettier fixture sentence would make every capture
    // evidence about copy the shipped page cannot produce.
    timeline: [
      entry(90, "node_running", "Step fix_batch running", { node_id: "fix_batch" }),
      entry(84, "node_succeeded", "Step review succeeded", { node_id: "review" }),
      entry(80, "generation_started", "Round 2 started"),
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

/** The settled run both terminal scenarios stage, spend included. */
const DONE: StoryReadState = {
  status: "done",
  progress: { round: 2, steps_done: 4, steps_total: 4 },
  spend: { tokens_used: 214_500, budget_tokens: 0 },
};

/** The done verdict, differing only in what survived retention. */
function doneVerdict(availability: LoopRunArtifact["availability"]): StoryVerdict {
  return {
    // `briefing.go` tones every non-failed terminal status `ok`.
    tone: "ok",
    headline: "The draft was rewritten and both review notes survived",
    detail: "Two rounds, 18m12s.",
    usage: { cost_usd: 0.87, duration: "18m12s" },
    outcome: { status: "done", cause: "verified", at: storyAt(47) },
    artifacts: [
      {
        name: "post-final.md",
        output: "write_artifacts",
        availability,
        // Pruned bytes have no digest left to cite.
        ...(availability === "pruned" ? {} : { ref: "sha256:2f81c4a9" }),
      },
    ],
  };
}

const DONE_ROSTER = [
  node("review", "succeeded"),
  node("fix_batch", "succeeded"),
  node("collect_fixes", "succeeded"),
  node("write_artifacts", "succeeded"),
];

/** VC-04: a finished run leading with its outcome and what it produced. */
export function registerDoneScenario(): LoopRunStoryScenario {
  return base({ ...readState(DONE, doneVerdict("available")), rosterNodes: DONE_ROSTER });
}

/**
 * VC-09: retention removed the bytes; the name and the fact survive.
 *
 * Restaged rather than patched on top of the done briefing: the previous
 * `as LoopBriefing` cast switched type checking off on the one object that is
 * supposed to prove a capture matches the generated read contract.
 */
export function registerPrunedArtifactScenario(): LoopRunStoryScenario {
  return base({ ...readState(DONE, doneVerdict("pruned")), rosterNodes: DONE_ROSTER });
}

/**
 * VC-08: the run failed after part of the join came back, and says so.
 *
 * `US-008.AC-3` is about outputs, not about the failure: some steps succeeded
 * before a terminal failure, so what they produced has to be labelled partial
 * rather than presented as the finished thing. Three signals carry it, all in
 * the default register — the artifact's warning `Partial` note under the
 * outcome, the join's warning `partial` chip, and the fan's `partial 7 of 10`
 * coverage. Each is a server-owned read: the artifact's `availability` comes
 * from the daemon's own output status (`run_read_briefing.go`), the join's state
 * from the roster, the coverage from the rollup. Nothing is computed here.
 */
export function registerPartialOutputsScenario(): LoopRunStoryScenario {
  const rollup: LoopFanoutRollup = {
    generation: 2,
    node_id: "fix_batches",
    done: 7,
    total: 10,
    failed: 3,
  };
  return base({
    ...readState(
      // Every action step settled — seven succeeded, three failed — which is
      // what the daemon counts as done. The run is over; the join is short.
      { status: "failed", progress: { round: 2, steps_done: 12, steps_total: 12 } },
      {
        tone: "failed",
        headline: "The join settled short and the run stopped there",
        detail:
          "Seven of ten fix workers came back before the join; three failed. The notes carry only what returned.",
        outcome: { status: "failed", cause: "join_incomplete", at: storyAt(2) },
        artifacts: [
          {
            name: "round-2-fixes.md",
            output: "collect_fixes",
            availability: "partial",
            ref: "sha256:9c02be41",
          },
        ],
      }
    ),
    rosterNodes: [
      node("review", "succeeded", { session_id: "ses-77120a3f" }),
      node("write_artifacts", "succeeded"),
      ...Array.from({ length: 10 }, (_unused, index) =>
        node("fix_batch", index < 7 ? "succeeded" : "failed", { item_index: index })
      ),
      // The join itself is what `partial` describes: it produced an output, and
      // the output is short of what the round asked for.
      node("collect_fixes", "partial"),
    ],
    rosterRollups: [rollup],
  });
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
      {
        status: "no-op",
        progress: { round: 1, steps_done: 0, steps_total: 0 },
        spend: { tokens_used: 0, budget_tokens: 0 },
      },
      {
        // A run that executed nothing is still tone `ok` — `briefing.go` reserves
        // `failed` for failed/exhausted/stalled.
        tone: "ok",
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
/**
 * The run this one forked to — the set's second canonical run id.
 *
 * `DESIGN-NOTES.md` fixes the data story at exactly two runs and forbids
 * minting a third, so the fork points at the one that already exists.
 */
export const LONG_STORY_FORK_RUN_ID = "looprun-77aa01b2c3d4e5f6";

/** The steps a round cycles through, so a page of history reads as a run. */
const LONG_STORY_NODES = ["review", "fix_batch", "collect_fixes", "write_artifacts"] as const;

/**
 * How many raw events the one folded heartbeat beat stands for.
 *
 * `coalesceTimeline` collapses consecutive heartbeat-class events — `token_tick`,
 * `runtime_applied`, `predicate_diagnostic` — into one entry spanning
 * `first_seq..seq`, and serves only that entry. Staging the fold rather than the
 * raw ticks is what makes the story pane show a coalesced beat at all; a fixture
 * of 600 individual step events produced a wall of identical rows and never
 * exercised the count once.
 */
const HEARTBEAT_SPAN = 142;

/**
 * Which beat is the folded one, counting from the newest.
 *
 * Deep enough that reaching it proves the story paged, close enough to the
 * paging control that one frame can hold both — the contract row is about a
 * long story that stays navigable, and evidence for that has to show the fold
 * and the way back in the same viewport.
 */
const HEARTBEAT_INDEX = 93;

function longStoryRound(seq: number): number {
  return seq > 400 ? 3 : seq > 200 ? 2 : 1;
}

function longStoryBeat(seq: number, index: number): LoopTimelineEntry {
  const generation = longStoryRound(seq);
  const at = storyAt(index);
  if (seq === 400 || seq === 200) {
    return entry(seq, "generation_started", `Round ${generation} started`, { at });
  }
  if (seq === 404) {
    return entry(seq, "gate_verdict", 'Approval "quality": approved', { generation, at });
  }
  const nodeId = LONG_STORY_NODES[seq % LONG_STORY_NODES.length];
  return seq % 3 === 0
    ? entry(seq, "node_succeeded", `Step ${nodeId} succeeded`, { generation, at, node_id: nodeId })
    : entry(seq, "node_running", `Step ${nodeId} running`, { generation, at, node_id: nodeId });
}

export function longStoryTimeline(): LoopTimelineEntry[] {
  const entries: LoopTimelineEntry[] = [];
  let seq = LONG_STORY_EVENT_COUNT;
  for (let index = 0; seq >= 2; index += 1) {
    if (index === HEARTBEAT_INDEX) {
      entries.push(
        entry(seq, "token_tick", "Token usage increased", {
          generation: longStoryRound(seq),
          at: storyAt(index),
          first_seq: seq - (HEARTBEAT_SPAN - 1),
        })
      );
      // The seqs the fold covers are never served as entries of their own.
      seq -= HEARTBEAT_SPAN;
      continue;
    }
    entries.push(longStoryBeat(seq, index));
    seq -= 1;
  }
  // The oldest beat is the fork point, so paging all the way back reaches the
  // one entry US-009.EC-3 is about. `timelineTitle` names the run the fork
  // produced; the "forked from" side is the lineage section's, not the story's.
  entries.push(
    entry(1, "run_forked", `Run forked to ${LONG_STORY_FORK_RUN_ID}`, {
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
