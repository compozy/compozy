import type { HomeOverview } from "../types";

export function makeHomeOverview(overrides: Partial<HomeOverview> = {}): HomeOverview {
  return {
    schema_version: "observe-overview/v1",
    generated_at: "2026-07-23T12:00:00Z",
    attention: {
      total: 2,
      by_kind: { approval: 1, failure: 1, needs_input: 0 },
      items: [
        {
          kind: "approval",
          title: "Deploy docs site",
          task_id: "task-approval",
          occurred_at: "2026-07-23T10:00:00Z",
          actions: ["approve", "reject", "open"],
        },
        {
          kind: "failure",
          title: "Overnight writer session",
          detail: "provider timed out",
          task_id: "task-failed",
          run_id: "run-failed",
          occurred_at: "2026-07-23T02:12:00Z",
          actions: ["retry", "open"],
        },
      ],
    },
    today: { runs_completed: 5, runs_failed: 1, tasks_closed: 3 },
    outcomes: {
      window_days: 14,
      days: [
        { date: "2026-07-22", completed: 6, failed: 1, canceled: 0 },
        { date: "2026-07-23", completed: 5, failed: 1, canceled: 1 },
      ],
      completed: 11,
      failed: 2,
      canceled: 1,
      success_pct: 78.6,
    },
    usage: {
      window_days: 30,
      retention_days: 7,
      truncated: true,
      total_tokens: 1_400_000,
      estimated_cost: 21.4,
      cost_currency: "USD",
      cost_status: "estimated",
      days: [
        { date: "2026-07-22", tokens: 700_000 },
        { date: "2026-07-23", tokens: 700_000 },
      ],
      agent_share: [
        { agent_name: "writer", tokens: 900_000, fraction: 0.64 },
        { agent_name: "researcher", tokens: 500_000, fraction: 0.36 },
      ],
      profiles: [
        {
          profile_id: "00000000000000000000000000",
          profile_name: "default",
          profile_color: "#8a8f98",
          profile_icon: "user-round",
          profile_archived: false,
          tokens: 900_000,
        },
        {
          profile_id: "01J9MARKETING00000000000000",
          profile_name: "marketing",
          profile_color: "#c26ad6",
          profile_icon: "megaphone",
          profile_archived: false,
          tokens: 400_000,
        },
        {
          profile_id: "01J9OLDAGENCY00000000000000",
          profile_name: "old agency",
          profile_color: "#b58e5f",
          profile_icon: "folder",
          profile_archived: true,
          tokens: 100_000,
        },
      ],
    },
    pulse: {
      window_days: 14,
      buckets: [{ weekday: 2, hour: 14, events: 42 }],
      busiest: { weekday: 2, hour: 14, events: 42 },
      longest_session: {
        session_id: "sess-long",
        agent_name: "writer",
        duration_seconds: 8040,
        date: "2026-07-22",
      },
    },
    network: { messages_today: 128 },
    system: { hook_runs_today: 24, hook_failures_today: 0, retention_days: 7 },
    freshness: {
      observed_at: "2026-07-23T12:00:00Z",
      latest_activity_at: "2026-07-23T11:59:00Z",
      age_ms: 60_000,
      stale_after_ms: 120_000,
      has_live_work: true,
      status: "current",
      stale: false,
    },
    ...overrides,
  };
}

/** A fresh install: every counter the overview carries is zero and nothing is running. */
export function makeEmptyHomeOverview(overrides: Partial<HomeOverview> = {}): HomeOverview {
  const base = makeHomeOverview();
  return {
    ...base,
    attention: { total: 0, by_kind: { approval: 0, failure: 0, needs_input: 0 }, items: [] },
    today: { runs_completed: 0, runs_failed: 0, tasks_closed: 0 },
    outcomes: { ...base.outcomes, days: [], completed: 0, failed: 0, canceled: 0, success_pct: 0 },
    usage: {
      ...base.usage,
      truncated: false,
      total_tokens: 0,
      estimated_cost: 0,
      days: [],
      agent_share: [],
    },
    pulse: { window_days: base.pulse.window_days, buckets: [] },
    network: { messages_today: 0 },
    system: { hook_runs_today: 0, hook_failures_today: 0, retention_days: 7 },
    freshness: { ...base.freshness, has_live_work: false },
    ...overrides,
  };
}
