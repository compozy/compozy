import type { OperationResponse } from "@/lib/api-contract";

import type { TaskRunNetworkProjection } from "../types";

type TaskRunRecord = OperationResponse<"getTaskRun", 200>["run"]["run"];

export function formatTaskRunBounds(
  run: TaskRunRecord,
  network: TaskRunNetworkProjection
): string | null {
  const participation = run.resolved_network_participation;
  if (participation?.mode !== "live") return null;
  const bounds = participation.bounds;
  const budget = network.usage.budget;
  const exhausted = budget?.exhausted_reason ? ` · exhausted: ${budget.exhausted_reason}` : "";
  return `Bounds: ${budget?.wakes_used ?? 0}/${bounds.max_wakes} wakes · wall ${budget?.wall_time_used ?? "0s"}/${bounds.max_total_wall_time} · tokens ${budget?.input_tokens_used ?? 0}/${bounds.max_input_tokens} in, ${budget?.output_tokens_used ?? 0}/${bounds.max_output_tokens} out · depth ≤${bounds.max_wake_depth}${exhausted}`;
}
