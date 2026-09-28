import type { SchedulerStatus } from "../types";

/** Queue counts shown in the controls meta line; a missing status reads as zero. */
export interface SchedulerStatusCounts {
  running: number;
  waiting: number;
  paused: number;
  starved: number;
  needsAttention: number;
}

export function schedulerStatusCounts(status: SchedulerStatus | null): SchedulerStatusCounts {
  return {
    running: status?.active_claim_count ?? 0,
    waiting: status?.queued_run_count ?? 0,
    paused: status?.paused_task_count ?? 0,
    starved: status?.starved_run_count ?? 0,
    needsAttention: status?.needs_attention_run_count ?? 0,
  };
}
