import {
  createFrameFactory,
  metricRatchetDefinition,
  metricRatchetRun,
  minutesAgo,
  nodePayload,
} from "./loop-run-page-fixture-world";
import { briefingFor } from "./loop-run-read-builders";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";
import type { LoopRunGeneration } from "../../types";

/**
 * Scenarios for the scored Loop — the `quality-ratchet` world.
 *
 * They live apart from the review-and-fix scenarios because a scored run is a
 * different subject: every state here turns on a gate verdict, a best
 * generation and a ratchet restore, and none of the review states have any of
 * those. `quality` is a control gate, so `draft` is each round's only action
 * step, which is what the staged progress counts say.
 */

function metricGeneration(
  generation: number,
  parentGeneration: number,
  origin: LoopRunGeneration["origin"],
  score?: number
): LoopRunGeneration {
  return {
    generation,
    parent_generation: parentGeneration,
    origin,
    route_causes: [],
    verdicts:
      score === undefined
        ? []
        : [
            {
              gate_id: "quality",
              item_index: 0,
              outcome: "approved",
              score,
              route_cause_rank: 0,
              criteria: [],
              blocking_issues: [],
            },
          ],
    outputs: [{ node_id: "draft", status: "succeeded", generation }],
  };
}

function metricVerdictPayload(generation: number, score: number, bestGeneration: number) {
  return {
    node_id: "quality",
    gate_id: "quality",
    generation,
    verdict: "pass",
    score,
    best_generation: bestGeneration,
    criteria: [{ id: "quality_score", type: "agent-judge", status: "pass", score }],
    blocking_issues: [],
    route: "next_generation",
  };
}

export function scoredBestScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory("r-score-best");
  const frames = [
    frame("generation_started", 12, {
      generation: 1,
      parent_generation: 0,
      origin: "initial",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 10, metricVerdictPayload(1, 0.72, 1)),
    frame("generation_started", 8, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_next_generation",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 5, metricVerdictPayload(2, 0.96, 2)),
    frame("status_changed", 4, {
      from: "running",
      to: "done",
      status: "done",
      cause: "stop_when",
    }),
  ];
  const run = metricRatchetRun({
    id: "r-score-best",
    status: "done",
    generation: 2,
    best_generation: 2,
    best_score: 0.96,
    progress: { round: 2, steps_done: 1, steps_total: 1 },
  });
  return {
    run,
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "The second candidate scored 0.96 and cleared the bar",
      detail: "Two rounds. The best generation is the one the run ended on.",
      outcome: { status: "done", cause: "stop_when", at: minutesAgo(4) },
    }),
    definition: metricRatchetDefinition,
    frames,
    generations: [
      metricGeneration(1, 0, "initial", 0.72),
      metricGeneration(2, 1, "gate_next_generation", 0.96),
    ],
  };
}

export function ratchetRestoreScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory("r-ratchet-restore");
  const frames = [
    frame("generation_started", 18, {
      generation: 1,
      parent_generation: 0,
      origin: "initial",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 16, metricVerdictPayload(1, 0.8, 1)),
    frame("generation_started", 13, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_next_generation",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 10, metricVerdictPayload(2, 0.6, 1)),
    frame("generation_started", 7, {
      generation: 3,
      parent_generation: 1,
      origin: "ratchet_restore",
      reattempt_strategy: "full_body",
    }),
    frame("node_running", 6, nodePayload("draft", 3)),
  ];
  const run = metricRatchetRun({
    id: "r-ratchet-restore",
    status: "running",
    generation: 3,
    best_generation: 1,
    best_score: 0.8,
    progress: { round: 3, steps_done: 0, steps_total: 1 },
  });
  return {
    run,
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Round 3 restarted from the best candidate so far",
      detail: "Round 2 scored lower than round 1, so the run went back to round 1's draft.",
    }),
    definition: metricRatchetDefinition,
    frames,
    generations: [
      metricGeneration(1, 0, "initial", 0.8),
      metricGeneration(2, 1, "gate_next_generation", 0.6),
      metricGeneration(3, 1, "ratchet_restore"),
    ],
  };
}

export function exhaustedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    frame("generation_started", 50, {
      generation: 1,
      parent_generation: 0,
      origin: "initial",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 44, metricVerdictPayload(1, 0.7, 1)),
    frame("generation_started", 40, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_next_generation",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 36, metricVerdictPayload(2, 0.6, 1)),
    frame("generation_started", 34, {
      generation: 3,
      parent_generation: 1,
      origin: "ratchet_restore",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 31, metricVerdictPayload(3, 0.5, 1)),
    frame("status_changed", 30, {
      from: "running",
      to: "exhausted",
      status: "exhausted",
      cause: "iteration_cap",
    }),
  ];
  const run = metricRatchetRun({
    id: "r-best-exhausted",
    status: "exhausted",
    generation: 3,
    best_generation: 1,
    best_score: 0.7,
    tokens_used: 144_000,
    created_at: minutesAgo(80),
    started_at: minutesAgo(80),
    last_progress_at: minutesAgo(30),
    progress: { round: 3, steps_done: 1, steps_total: 1 },
  });
  return {
    run,
    briefing: briefingFor(run, {
      tone: "failed",
      headline: "Three rounds spent and the bar was never reached",
      detail: "The best candidate is round 1's, at 0.7. The loop allows three rounds.",
      outcome: { status: "exhausted", cause: "iteration_cap", at: minutesAgo(30) },
    }),
    definition: metricRatchetDefinition,
    frames,
    generations: [
      metricGeneration(1, 0, "initial", 0.7),
      metricGeneration(2, 1, "gate_next_generation", 0.6),
      metricGeneration(3, 1, "ratchet_restore", 0.5),
    ],
  };
}
