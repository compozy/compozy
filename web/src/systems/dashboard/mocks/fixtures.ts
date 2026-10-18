import { storyAgentNames, storySessionIds, storyWorkspaceIds } from "@/storybook/fintech-scenario";
import {
  consultingProfileFixture,
  defaultProfileFixture,
  marketingProfileFixture,
  oldAgencyProfileFixture,
} from "@/systems/profiles/mocks";
import type { ProfilePayload } from "@/systems/profiles";

import type { HomeActivityEvent, HomeOverview, HomeProfileUsage } from "../types";

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
      profiles: [],
    },
    pulse: { window_days: base.pulse.window_days, buckets: [] },
    system: { hook_runs_today: 0, hook_failures_today: 0, retention_days: 7 },
    freshness: { ...base.freshness, has_live_work: false },
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Route-story fixtures: the Northstar Pay launch-week scenario the rest of the
// Storybook mocks share, so the home overview names the same agents, sessions,
// tasks, and profiles the other surfaces render.
// ---------------------------------------------------------------------------

const STORY_NOW = "2026-04-17T18:30:00Z";
const STORY_DAY_MS = 24 * 60 * 60 * 1000;

function storyDate(daysAgo: number): string {
  return new Date(Date.parse(STORY_NOW) - daysAgo * STORY_DAY_MS).toISOString().slice(0, 10);
}

function storyTimestamp(minutesAgo: number): string {
  return new Date(Date.parse(STORY_NOW) - minutesAgo * 60_000).toISOString();
}

function storyProfileUsage(profile: ProfilePayload, tokens: number): HomeProfileUsage {
  return {
    profile_id: profile.id,
    profile_name: profile.name,
    profile_color: profile.color,
    profile_icon: profile.icon ?? undefined,
    profile_archived: profile.state === "archived",
    tokens,
  };
}

const STORY_USAGE_DAYS = Array.from({ length: 30 }, (_, index) => ({
  date: storyDate(29 - index),
  tokens: 42_000 + ((index * 37) % 11) * 9_500 + (index > 22 ? 60_000 : 0),
}));
const STORY_USAGE_TOTAL = STORY_USAGE_DAYS.reduce((sum, day) => sum + day.tokens, 0);

const STORY_OUTCOME_DAYS = Array.from({ length: 14 }, (_, index) => {
  const completed = 4 + ((index * 5) % 7);
  const failed = index % 4 === 1 ? 1 : 0;
  const canceled = index % 6 === 3 ? 1 : 0;
  return { date: storyDate(13 - index), completed, failed, canceled };
});
const STORY_OUTCOME_TOTALS = STORY_OUTCOME_DAYS.reduce(
  (totals, day) => ({
    completed: totals.completed + day.completed,
    failed: totals.failed + day.failed,
    canceled: totals.canceled + day.canceled,
  }),
  { completed: 0, failed: 0, canceled: 0 }
);

const STORY_PULSE_BUCKETS = Array.from({ length: 7 * 24 }, (_, index) => {
  const weekday = Math.floor(index / 24);
  const hour = index % 24;
  const working = weekday >= 1 && weekday <= 5 && hour >= 9 && hour <= 19;
  const events = working ? 8 + ((weekday * 31 + hour * 17) % 15) : hour >= 11 && hour <= 16 ? 3 : 0;
  return { weekday, hour, events: weekday === 5 && hour === 17 ? 58 : events };
});

/** Populated home overview for the default route story (Friday of launch week). */
export const homeOverviewFixture: HomeOverview = makeHomeOverview({
  generated_at: STORY_NOW,
  attention: {
    total: 3,
    by_kind: { approval: 1, failure: 1, needs_input: 1 },
    items: [
      {
        kind: "approval",
        title: "Approve public timeout copy for BR merchants",
        detail: `${storyAgentNames.compliance} is waiting on a manual approval`,
        task_id: "task_006",
        run_id: "run_006",
        occurred_at: storyTimestamp(12),
        actions: ["approve", "reject", "open"],
      },
      {
        kind: "needs_input",
        title: "Confirm the MX cashback wording before the landing page ships",
        detail: `${storyAgentNames.copywriter} asked which claim variant legal cleared`,
        session_id: storySessionIds.copywriter,
        occurred_at: storyTimestamp(27),
        actions: ["open"],
      },
      {
        kind: "failure",
        title: "Reconcile BR settlement replay ETA with the partner bank",
        detail: "partner-bank replay detail was stale and blocked the public timeout copy",
        task_id: "task_004",
        run_id: "run_004",
        occurred_at: storyTimestamp(76),
        actions: ["retry", "open"],
      },
    ],
  },
  today: { runs_completed: 14, runs_failed: 1, tasks_closed: 9 },
  outcomes: {
    window_days: 14,
    days: STORY_OUTCOME_DAYS,
    ...STORY_OUTCOME_TOTALS,
    success_pct:
      Math.round(
        (STORY_OUTCOME_TOTALS.completed /
          (STORY_OUTCOME_TOTALS.completed +
            STORY_OUTCOME_TOTALS.failed +
            STORY_OUTCOME_TOTALS.canceled)) *
          1000
      ) / 10,
  },
  usage: {
    window_days: 30,
    retention_days: 60,
    truncated: false,
    total_tokens: STORY_USAGE_TOTAL,
    estimated_cost: Math.round((STORY_USAGE_TOTAL / 1_000_000) * 15 * 100) / 100,
    cost_currency: "USD",
    cost_status: "estimated",
    days: STORY_USAGE_DAYS,
    agent_share: [
      {
        agent_name: storyAgentNames.release,
        tokens: Math.round(STORY_USAGE_TOTAL * 0.34),
        fraction: 0.34,
      },
      {
        agent_name: storyAgentNames.platform,
        tokens: Math.round(STORY_USAGE_TOTAL * 0.27),
        fraction: 0.27,
      },
      {
        agent_name: storyAgentNames.copywriter,
        tokens: Math.round(STORY_USAGE_TOTAL * 0.18),
        fraction: 0.18,
      },
      {
        agent_name: storyAgentNames.fraud,
        tokens: Math.round(STORY_USAGE_TOTAL * 0.12),
        fraction: 0.12,
      },
      {
        agent_name: storyAgentNames.support,
        tokens: Math.round(STORY_USAGE_TOTAL * 0.09),
        fraction: 0.09,
      },
    ],
    profiles: [
      storyProfileUsage(defaultProfileFixture, Math.round(STORY_USAGE_TOTAL * 0.58)),
      storyProfileUsage(marketingProfileFixture, Math.round(STORY_USAGE_TOTAL * 0.27)),
      storyProfileUsage(consultingProfileFixture, Math.round(STORY_USAGE_TOTAL * 0.11)),
      storyProfileUsage(oldAgencyProfileFixture, Math.round(STORY_USAGE_TOTAL * 0.04)),
    ],
  },
  pulse: {
    window_days: 14,
    buckets: STORY_PULSE_BUCKETS,
    busiest: { weekday: 5, hour: 17, events: 58 },
    longest_session: {
      session_id: storySessionIds.release,
      agent_name: storyAgentNames.release,
      duration_seconds: 11_640,
      date: storyDate(1),
    },
  },
  system: { hook_runs_today: 38, hook_failures_today: 1, retention_days: 60 },
  freshness: {
    observed_at: STORY_NOW,
    latest_activity_at: storyTimestamp(1),
    age_ms: 60_000,
    stale_after_ms: 120_000,
    has_live_work: true,
    status: "current",
    stale: false,
  },
});

function storyActivityEvent(
  event: Pick<HomeActivityEvent, "id" | "type" | "agent_name" | "session_id" | "summary"> &
    Partial<HomeActivityEvent> & { minutesAgo: number }
): HomeActivityEvent {
  const { minutesAgo, ...rest } = event;
  return {
    profile_id: defaultProfileFixture.id,
    profile_name: defaultProfileFixture.name,
    profile_color: defaultProfileFixture.color,
    profile_icon: defaultProfileFixture.icon ?? undefined,
    profile_archived: false,
    spawn_depth: 0,
    workspace_id: storyWorkspaceIds.hq,
    timestamp: storyTimestamp(minutesAgo),
    ...rest,
  };
}

/** Recent lifecycle activity for the home feed, newest first. */
export const homeActivityFixture: HomeActivityEvent[] = [
  storyActivityEvent({
    id: "evt_home_01",
    type: "task.approval_requested",
    outcome: "warning",
    agent_name: storyAgentNames.compliance,
    session_id: storySessionIds.compliance,
    task_id: "task_006",
    run_id: "run_006",
    summary: "Approval requested: public timeout copy for BR merchants",
    minutesAgo: 12,
  }),
  storyActivityEvent({
    id: "evt_home_02",
    type: "session.completed",
    outcome: "success",
    agent_name: storyAgentNames.frontend,
    session_id: storySessionIds.frontend,
    summary: "Mobile hero QA passed on 6 of 6 breakpoints",
    minutesAgo: 19,
  }),
  storyActivityEvent({
    id: "evt_home_03",
    type: "session.input_requested",
    outcome: "warning",
    agent_name: storyAgentNames.copywriter,
    session_id: storySessionIds.copywriter,
    summary: "Asked which MX cashback claim variant legal cleared",
    minutesAgo: 27,
  }),
  storyActivityEvent({
    id: "evt_home_04",
    type: "task.run_started",
    agent_name: storyAgentNames.release,
    session_id: storySessionIds.release,
    task_id: "task_009",
    summary: "Canary 25% rollout for checkout release started",
    minutesAgo: 41,
  }),
  storyActivityEvent({
    id: "evt_home_05",
    type: "tool.call_completed",
    outcome: "success",
    agent_name: storyAgentNames.fraud,
    session_id: storySessionIds.fraud,
    summary: "Queried reserve exposure for the pilot merchant batch",
    minutesAgo: 52,
  }),
  storyActivityEvent({
    id: "evt_home_06",
    type: "task.run_failed",
    outcome: "failure",
    agent_name: storyAgentNames.platform,
    session_id: storySessionIds.platform,
    task_id: "task_004",
    run_id: "run_004",
    summary: "Settlement replay reconciliation failed after 3 attempts",
    minutesAgo: 76,
  }),
  storyActivityEvent({
    id: "evt_home_07",
    type: "task.closed",
    outcome: "success",
    agent_name: storyAgentNames.support,
    session_id: storySessionIds.support,
    summary: "Support macros updated for launch-day pricing questions",
    minutesAgo: 94,
  }),
];
