import type { LoopDetail, LoopRun, LoopRunDetail, LoopRunGeneration } from "../types";

// Generations aligned to the delivery graph so the run-page timeline resolves
// each node's real class/kind. G1 has one failed branch; G2 carries the clean
// branches and re-runs only the failed one.
function deliveryGenerations(run: LoopRun): LoopRunGeneration[] {
  const live = run.status === "running" || run.status === "needs-approval";
  const g1: LoopRunGeneration = {
    generation: 1,
    parent_generation: 0,
    origin: "initial",
    verdicts: [],
    outputs: [
      { node_id: "slug", status: "succeeded", generation: 1 },
      { node_id: "load_tasks", status: "succeeded", generation: 1 },
      {
        node_id: "execute_task",
        status: "succeeded",
        generation: 1,
        item_index: 0,
        task_run_id: "tr_1",
      },
      {
        node_id: "execute_task",
        status: "succeeded",
        generation: 1,
        item_index: 1,
        task_run_id: "tr_2",
      },
      {
        node_id: "execute_task",
        status: "failed",
        generation: 1,
        item_index: 2,
        task_run_id: "tr_3",
        resolved_runtime: {
          provider: "openai",
          model: "gpt-5.4",
          reasoning: "high",
          source: { provider: "run", model: "frontmatter", reasoning: "config" },
        },
      },
      { node_id: "collect", status: "succeeded", generation: 1 },
      { node_id: "review", status: "failed", generation: 1 },
    ],
  };
  const g2: LoopRunGeneration = {
    generation: 2,
    parent_generation: 1,
    origin: "gate_revise",
    verdicts:
      run.best_generation === 2
        ? [
            {
              gate_id: "review",
              outcome: "rejected",
              score: run.best_score,
              criteria: [],
              blocking_issues: [],
            },
          ]
        : [],
    outputs: [
      { node_id: "execute_task", status: "reused", generation: 2, item_index: 0 },
      { node_id: "execute_task", status: "reused", generation: 2, item_index: 1 },
      {
        node_id: "execute_task",
        status: live ? "running" : "succeeded",
        generation: 2,
        item_index: 2,
        resolved_runtime: {
          provider: "openai",
          model: "gpt-5.4",
          reasoning: "high",
          source: { provider: "run", model: "frontmatter", reasoning: "config" },
        },
      },
      {
        node_id: "child_delivery",
        status: "awaiting_child",
        generation: 2,
        child_loop_run_id: "looprun_child",
      },
      { node_id: "collect", status: live ? "pending" : "succeeded", generation: 2 },
      { node_id: "review", status: live ? "pending" : "succeeded", generation: 2 },
      { node_id: "verify", status: live ? "pending" : "succeeded", generation: 2 },
      { node_id: "approve", status: live ? "pending" : "succeeded", generation: 2 },
    ],
  };
  const generations = run.generation >= 2 ? [g1, g2] : [g1];
  const referenced = new Set(generations.map(generation => generation.generation));
  for (const generation of [run.best_generation, run.generation]) {
    if (!generation || referenced.has(generation)) continue;
    generations.push({
      generation,
      parent_generation: Math.max(0, generation - 1),
      origin: run.best_generation === generation ? "ratchet_restore" : "gate_revise",
      verdicts:
        run.best_generation === generation
          ? [
              {
                gate_id: "review",
                outcome: "rejected",
                score: run.best_score,
                criteria: [],
                blocking_issues: [],
              },
            ]
          : [],
      outputs: [],
    });
    referenced.add(generation);
  }
  return generations.sort((left, right) => left.generation - right.generation);
}

function reviewGenerations(run: LoopRun): LoopRunGeneration[] {
  const active = run.status === "running";
  const waiting = active || run.status === "paused";
  const remainingStatus = waiting ? "pending" : "succeeded";
  const generation = Math.max(1, run.generation);
  return [
    {
      generation,
      parent_generation: Math.max(0, generation - 1),
      origin: generation > 1 ? "gate_revise" : "initial",
      verdicts: [],
      outputs: [
        { node_id: "review", status: "succeeded", generation },
        { node_id: "has_issues", status: "succeeded", generation },
        { node_id: "write_artifacts", status: "succeeded", generation },
        {
          node_id: "fix_batch",
          status: active ? "running" : remainingStatus,
          generation,
          item_index: 0,
        },
        { node_id: "collect_fixes", status: remainingStatus, generation },
        { node_id: "finalize_round", status: remainingStatus, generation },
      ],
    },
  ];
}

export function buildLoopRunDetailFixtures(
  runs: readonly LoopRun[],
  detailsByName: ReadonlyMap<string, LoopDetail>
): LoopRunDetail[] {
  return runs.map(run => {
    const detail = detailsByName.get(run.loop_name);
    if (!detail) throw new Error(`Missing Loop detail fixture for ${run.loop_name}`);
    return {
      run: {
        ...run,
        started_by_kind: "user",
        started_by_ref: "operator",
        started_origin_kind: run.started_origin_kind ?? "cli",
      },
      executed_definition: detail.definition,
      generations:
        run.loop_name === "review-and-fix" ? reviewGenerations(run) : deliveryGenerations(run),
    };
  });
}
