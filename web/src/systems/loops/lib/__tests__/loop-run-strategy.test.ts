import { describe, expect, it } from "vitest";

import { releaseTrainDetail } from "../../mocks/fixture-release-train";
import {
  releaseTrainPartialRun,
  releaseTrainPartialRunDetail,
} from "../../mocks/fixture-graph-eng-runs";
import { readLoopGraph, type LoopGraph } from "../loop-graph";
import {
  buildStrategyProgress,
  LOOP_STRATEGY_WIDE_THRESHOLD,
  type LoopStrategyProgressInput,
} from "../loop-run-strategy";
import type { LoopDefinition, LoopRunGeneration } from "../../types";

function graphOf(
  nodes: Record<string, unknown>[],
  edges: { from: string; to: string }[]
): LoopGraph {
  return readLoopGraph({ graph: { nodes, edges } } as unknown as Pick<LoopDefinition, "graph">);
}

function fanOutGraph(fanOut: Record<string, unknown> = {}): LoopGraph {
  return graphOf(
    [
      { id: "src", class: "source", kind: "input" },
      { id: "spread", class: "control", kind: "fan-out", ...fanOut },
      { id: "work", class: "action", kind: "run-agent" },
      { id: "join", class: "control", kind: "collect" },
    ],
    [
      { from: "src", to: "spread" },
      { from: "spread", to: "work" },
      { from: "work", to: "join" },
    ]
  );
}

function generationOf(outputs: LoopRunGeneration["outputs"]): LoopRunGeneration {
  return {
    generation: 1,
    parent_generation: 0,
    origin: "initial",
    route_causes: [],
    verdicts: [],
    outputs,
  };
}

function lanes(statuses: string[]): LoopRunGeneration["outputs"] {
  return statuses.map((status, index) => ({
    node_id: "work",
    status,
    generation: 1,
    item_index: index,
  }));
}

function progressFor(
  input: Partial<LoopStrategyProgressInput> & { graph: LoopGraph }
): ReturnType<typeof buildStrategyProgress> {
  return buildStrategyProgress({
    run: { completion_state: "complete", generation: 1 },
    generations: [generationOf([])],
    frames: [],
    ...input,
  });
}

describe("buildStrategyProgress", () => {
  it("Should count a strategy-canceled lane apart from a failed one", () => {
    const [model] = progressFor({
      graph: fanOutGraph(),
      generations: [generationOf(lanes(["succeeded", "failed", "canceled", "running", "queued"]))],
    });
    expect(model.counts).toMatchObject({
      succeeded: 1,
      failed: 1,
      canceledByStrategy: 1,
      active: 1,
      pending: 1,
      opened: 5,
      settled: 3,
    });
  });

  it("Should derive never-materialized lanes from the declared width minus the opened ones", () => {
    const [declared] = progressFor({
      graph: fanOutGraph({ max_fan_out: 5 }),
      generations: [generationOf(lanes(["succeeded", "running"]))],
    });
    expect(declared.counts).toMatchObject({ opened: 2, neverMaterialized: 3 });

    const [undeclared] = progressFor({
      graph: fanOutGraph(),
      generations: [generationOf(lanes(["succeeded", "running"]))],
    });
    expect(undeclared.counts.neverMaterialized).toBe(0);

    const [overspill] = progressFor({
      graph: fanOutGraph({ max_fan_out: 1 }),
      generations: [generationOf(lanes(["succeeded", "succeeded", "succeeded"]))],
    });
    expect(overspill.counts.neverMaterialized).toBe(0);
  });

  it("Should read partial from the run boundary or the collect cell, never from the counts", () => {
    const failedLanes = [generationOf(lanes(["succeeded", "failed"]))];

    const fromRunBoundary = progressFor({
      graph: fanOutGraph(),
      run: { completion_state: "partial", generation: 1 },
      generations: failedLanes,
    })[0];
    expect(fromRunBoundary.completionState).toBe("partial");
    expect(fromRunBoundary.isPartial).toBe(true);

    const fromCollectCell = progressFor({
      graph: fanOutGraph(),
      generations: [
        generationOf([
          ...lanes(["succeeded", "failed"]),
          { node_id: "join", status: "partial", generation: 1 },
        ]),
      ],
    })[0];
    expect(fromCollectCell.completionState).toBe("complete");
    expect(fromCollectCell.isPartial).toBe(true);

    const complete = progressFor({ graph: fanOutGraph(), generations: failedLanes })[0];
    expect(complete.isPartial).toBe(false);

    const unknown = progressFor({
      graph: fanOutGraph(),
      run: { completion_state: "half-done", generation: 1 },
      generations: failedLanes,
    })[0];
    expect(unknown.completionState).toBe("complete");
  });

  it("Should exclude strategy-canceled lanes from the coverage denominator", () => {
    const [model] = progressFor({
      graph: fanOutGraph(),
      generations: [
        generationOf(lanes(["succeeded", "succeeded", "failed", "canceled", "skipped"])),
      ],
    });

    expect(model.coverageLabel).toBe("2 of 3 lanes");
    expect(model.coverageRate).toBeCloseTo(2 / 3);

    const [single] = progressFor({
      graph: fanOutGraph(),
      generations: [generationOf(lanes(["succeeded"]))],
    });
    expect(single.coverageLabel).toBe("1 of 1 lane");

    const [none] = progressFor({ graph: fanOutGraph(), generations: [generationOf([])] });
    expect(none.coverageLabel).toBe("no lanes");
    expect(none.coverageRate).toBe(0);
  });

  it("Should flip to the aggregate-only reading past the wide threshold", () => {
    const atThreshold = progressFor({
      graph: fanOutGraph({ max_fan_out: LOOP_STRATEGY_WIDE_THRESHOLD }),
      generations: [generationOf([])],
    })[0];
    expect(atThreshold.isWide).toBe(false);

    const past = progressFor({
      graph: fanOutGraph({ max_fan_out: LOOP_STRATEGY_WIDE_THRESHOLD + 1 }),
      generations: [generationOf([])],
    })[0];
    expect(past.isWide).toBe(true);
  });

  it("Should return an empty list for a loop that fans nothing out", () => {
    const linear = graphOf(
      [
        { id: "src", class: "source", kind: "input" },
        { id: "work", class: "action", kind: "run-agent" },
      ],
      [{ from: "src", to: "work" }]
    );
    expect(progressFor({ graph: linear })).toEqual([]);
    expect(progressFor({ graph: null as unknown as LoopGraph })).toEqual([]);
  });

  it("Should leave the strategy label null when the author declared none", () => {
    const [undeclared] = progressFor({ graph: fanOutGraph() });
    expect(undeclared.strategyLabel).toBeNull();
    expect(undeclared.strategyKind).toBe("wait_all");
    expect(undeclared.threshold).toBeNull();
    expect(undeclared.missingAcceptable).toBe(false);

    const [declared] = progressFor({
      graph: fanOutGraph({
        strategy: { kind: "best_effort", threshold: "66%", missing: "acceptable" },
      }),
    });
    expect(declared.strategyLabel).toBe("best_effort 66% missing acceptable");
    expect(declared.strategyKind).toBe("best_effort");
    expect(declared.threshold).toBe("66%");
    expect(declared.missingAcceptable).toBe(true);
  });

  it("Should name the fan-out and the collect node its lanes join into", () => {
    const [model] = progressFor({ graph: fanOutGraph() });
    expect(model.nodeId).toBe("spread");
    expect(model.joinNodeId).toBe("join");
  });

  it("Should derive the release-train panel from the newest generation's own rows", () => {
    const [model] = buildStrategyProgress({
      run: releaseTrainPartialRun,
      graph: readLoopGraph(releaseTrainDetail.definition),
      generations: releaseTrainPartialRunDetail.generations ?? [],
      frames: [],
    });
    expect(model.nodeId).toBe("rollout");
    expect(model.joinNodeId).toBe("collect-rollout");
    expect(model.counts).toMatchObject({
      succeeded: 1,
      failed: 1,
      canceledByStrategy: 1,
      opened: 3,

      neverMaterialized: 497,
    });
    expect(model.coverageLabel).toBe("1 of 2 lanes");
    expect(model.isPartial).toBe(true);
    expect(model.isWide).toBe(true);
  });
});
