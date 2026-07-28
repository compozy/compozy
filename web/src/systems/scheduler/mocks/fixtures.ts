import type { SchedulerBacklog, SchedulerDrainResult, SchedulerStatus } from "../types";

export const schedulerStatusFixture: SchedulerStatus = {
  active_claim_count: 1,
  as_of: "2026-04-17T18:02:00Z",
  needs_attention_run_count: 0,
  paused: false,
  paused_task_count: 1,
  queued_run_count: 3,
  starved_run_count: 0,
};

export const schedulerPausedStatusFixture: SchedulerStatus = {
  ...schedulerStatusFixture,
  active_claim_count: 0,
  paused: true,
  paused_at: "2026-04-17T18:00:00Z",
  paused_by: "human:storybook",
  paused_reason: "provider incident",
};

export const schedulerAttentionStatusFixture: SchedulerStatus = {
  ...schedulerStatusFixture,
  needs_attention_run_count: 1,
  starved_run_count: 2,
};

export const schedulerBacklogFixture = {
  total: 2,
  runs: [
    {
      task: {
        id: "task_014",
        identifier: "TASK-14",
        scope: "workspace",
        status: "in_progress",
        title: "Verify webhook replay backlog at the partner bank",
        priority: "high",
        effective_paused: false,
        paused: false,
      },
      run: {
        id: "run_014",
        task_id: "task_014",
        attempt: 1,
        status: "queued",
        queued_at: "2026-04-17T17:44:00Z",
      },
    },
    {
      task: {
        id: "task_018",
        identifier: "TASK-18",
        scope: "workspace",
        status: "ready",
        title: "Finalize launch-room owner matrix and escalation routing",
        priority: "low",
        effective_paused: true,
        paused: true,
        paused_by_task_id: "task_018",
      },
      run: {
        id: "run_018",
        task_id: "task_018",
        attempt: 1,
        status: "queued",
        queued_at: "2026-04-17T17:49:00Z",
      },
    },
  ],
} as SchedulerBacklog;

export const schedulerDrainResultFixture: SchedulerDrainResult = {
  completed: true,
  completed_at: "2026-04-17T18:02:10Z",
  remaining_claims: 0,
  scheduler: schedulerPausedStatusFixture,
  started_at: "2026-04-17T18:02:00Z",
};
