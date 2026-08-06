import type { LoopDetail, LoopRun, LoopRunDetail, LoopRunGeneration } from "../types";

function implementTasksGenerations(run: LoopRun): LoopRunGeneration[] {
  if (run.generation === 0) return [];
  const generation = Math.max(1, run.generation);
  const active = run.status === "running";
  const failed = run.status === "failed";
  return [
    {
      generation,
      parent_generation: Math.max(0, generation - 1),
      origin: "initial",
      verdicts: [],
      outputs: [
        { node_id: "slug_input", status: "succeeded", generation },
        { node_id: "load_tasks", status: "succeeded", generation },
        { node_id: "implement", status: "succeeded", generation },
        {
          node_id: "execute_task",
          status: active ? "running" : failed ? "failed" : "succeeded",
          generation,
          item_index: 0,
          task_run_id: "tr_1",
          resolved_runtime: {
            provider: "openai",
            model: "gpt-5.4",
            reasoning: "high",
            source: { provider: "run", model: "frontmatter", reasoning: "config" },
          },
        },
        { node_id: "collect", status: active || failed ? "pending" : "succeeded", generation },
      ],
    },
  ];
}

function qualityGateGenerations(run: LoopRun): LoopRunGeneration[] {
  const live = run.status === "needs-approval";
  const g1: LoopRunGeneration = {
    generation: 1,
    parent_generation: 0,
    origin: "initial",
    verdicts: [],
    outputs: [
      { node_id: "slug", status: "succeeded", generation: 1 },
      { node_id: "load_tasks", status: "succeeded", generation: 1 },
      { node_id: "implement", status: "succeeded", generation: 1 },
      { node_id: "execute_task", status: "succeeded", generation: 1, item_index: 0 },
      { node_id: "collect", status: "succeeded", generation: 1 },
      { node_id: "review", status: "failed", generation: 1 },
    ],
  };
  const g2: LoopRunGeneration = {
    generation: 2,
    parent_generation: 1,
    origin: "gate_revise",
    verdicts: [
      {
        gate_id: "review",
        item_index: 0,
        outcome: "rejected",
        score: run.best_score,
        criteria: [],
        blocking_issues: [],
      },
    ],
    outputs: [
      { node_id: "slug", status: "reused", generation: 2 },
      { node_id: "load_tasks", status: "reused", generation: 2 },
      { node_id: "implement", status: "reused", generation: 2 },
      { node_id: "execute_task", status: "reused", generation: 2, item_index: 0 },
      { node_id: "collect", status: "reused", generation: 2 },
      { node_id: "review", status: "succeeded", generation: 2 },
      { node_id: "verify", status: "succeeded", generation: 2 },
      { node_id: "approve", status: live ? "pending" : "succeeded", generation: 2 },
    ],
  };
  const generations = run.generation >= 2 ? [g1, g2] : [g1];
  const finalGeneration = Math.max(run.generation, run.best_generation ?? 0);
  for (let generation = 3; generation <= finalGeneration; generation += 1) {
    generations.push({
      generation,
      parent_generation: generation - 1,
      origin: run.best_generation === generation ? "ratchet_restore" : "gate_revise",
      verdicts: [],
      outputs: [],
    });
  }
  return generations;
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
        { node_id: "fix_batches", status: "succeeded", generation },
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
        run.loop_name === "review-and-fix"
          ? reviewGenerations(run)
          : run.loop_name === "quality-gate-demo"
            ? qualityGateGenerations(run)
            : implementTasksGenerations(run),
      node_controls: [],
      waits: [],
    };
  });
}
