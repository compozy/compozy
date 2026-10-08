/**
 * Detail-page fixtures from the board's shared story (`DESIGN-NOTES.md`):
 * workspace checkout-api, its schedules, events and the deploy link, plus the
 * production run fixtures. Shared by the detail Vitest suites and stories.
 */

import type { AutomationJob, AutomationRun, AutomationTrigger } from "../types";

const PROFILE = { profile_id: "00000000000000000000000000", profile_name: "default" } as const;
const WORKSPACE_ID = "ws_checkout_api";

export const detailWorkspaces = [{ id: WORKSPACE_ID, name: "checkout-api" }];

const jobBase: AutomationJob = {
  ...PROFILE,
  id: "morning-digest",
  name: "morning-digest",
  agent_name: "summarizer",
  prompt: [
    "Summarize yesterday's sessions in checkout-api for the team channel.",
    "Group them by outcome: finished, failed, still waiting on someone.",
    "For every failure, name the likely cause in one line and link the session.",
    "End with the approvals that are still pending and who they are waiting on.",
  ].join("\n"),
  schedule: { mode: "cron", expr: "0 9 * * 1-5", catch_up_policy: "skip_missed" },
  scheduler: {
    job_id: "morning-digest",
    registered: true,
    misfire_count: 1,
    misfire_grace_seconds: 30,
    last_fire_id: "fire_morning_digest_118",
    next_run_at: "2026-10-08T09:00:00Z",
    catch_up_policy: "skip_missed",
  },
  next_run: "2026-10-08T09:00:00Z",
  scope: "workspace",
  workspace_id: WORKSPACE_ID,
  source: "dynamic",
  target_kind: "agent",
  enabled: true,
  retry: { strategy: "none", max_retries: 0, base_delay: "" },
  fire_limit: { max: 12, window: "1h" },
  created_at: "2026-09-29T10:00:00Z",
  updated_at: "2026-10-06T16:02:00Z",
};

export function makeDetailJob(overrides: Partial<AutomationJob> = {}): AutomationJob {
  return { ...jobBase, ...overrides };
}

export const morningDigestJob = makeDetailJob();

export const dependencyReviewJob = makeDetailJob({
  id: "dependency-review",
  name: "dependency-review",
  agent_name: "",
  prompt: "",
  schedule: { mode: "cron", expr: "0 8 * * 1" },
  scheduler: { job_id: "dependency-review", registered: false },
  next_run: null,
  enabled: false,
  task: {
    title: "Review dependency updates",
    owner: { kind: "pool", ref: "reviewers" },
  },
  updated_at: "2026-10-01T12:00:00Z",
});

export const releaseChecklistJob = makeDetailJob({
  id: "release-checklist",
  name: "release-checklist",
  agent_name: "release-manager",
  prompt: "Check the release checklist.",
  schedule: { mode: "every", interval: "30m" },
  scheduler: { job_id: "release-checklist", registered: true },
  scope: "global",
  workspace_id: undefined,
  source: "config",
});

const triggerBase: AutomationTrigger = {
  ...PROFILE,
  id: "rerun-delivery",
  name: "rerun-delivery",
  agent_name: "",
  prompt: "",
  event: "session.stopped",
  filter: { "data.stop_reason": "error" },
  scope: "workspace",
  workspace_id: WORKSPACE_ID,
  source: "dynamic",
  target_kind: "loop",
  loop_target: {
    workspace_id: WORKSPACE_ID,
    loop_name: "software-delivery",
    inputs: { implementer: "code_implementer", target_branch: "main" },
    input_mapping: { slug: "data.session_name" },
  },
  enabled: true,
  retry: { strategy: "backoff", max_retries: 2, base_delay: "5s" },
  fire_limit: { max: 4, window: "1h" },
  webhook_secret_present: false,
  created_at: "2026-09-25T09:00:00Z",
  updated_at: "2026-10-04T09:00:00Z",
};

export function makeDetailTrigger(overrides: Partial<AutomationTrigger> = {}): AutomationTrigger {
  return { ...triggerBase, ...overrides };
}

export const rerunDeliveryTrigger = makeDetailTrigger();

export const summarizeFailuresTrigger = makeDetailTrigger({
  id: "summarize-failures",
  name: "summarize-failures",
  agent_name: "summarizer",
  prompt: 'Session {{ .Data.session_id }} stopped with reason "{{ .Data.stop_reason }}".',
  target_kind: "agent",
  loop_target: null,
});

export const deployWebhookTrigger = makeDetailTrigger({
  id: "deploy-webhook",
  name: "deploy-webhook",
  agent_name: "deployer",
  prompt:
    "Deploy {{ .Data.branch }} of checkout-api to production.\nPost the release notes and roll back if the health check fails.",
  event: "webhook",
  filter: { "data.action": "deploy", "data.branch": "main" },
  endpoint_slug: "deploy",
  webhook_id: "wbh_abc123",
  webhook_secret_present: true,
  target_kind: "agent",
  loop_target: null,
  ingress: {
    confirmed_at: "2026-10-04T09:00:00Z",
    confirmed_endpoint_generation: 4,
    endpoint_generation: 4,
    reachability: "live",
    scope_kind: "workspace",
    subject_id: "deploy-webhook",
    subject_kind: "webhook_trigger",
    url: "https://public.gateway.test/api/webhooks/workspaces/ws_checkout_api/deploy--wbh_abc123",
    workspace_id: WORKSPACE_ID,
  },
});

const runBase: AutomationRun = {
  ...PROFILE,
  id: "run_001",
  status: "completed",
  attempt: 1,
  job_id: "morning-digest",
  fire_id: "fire_morning_digest_117",
  scheduled_at: "2026-10-07T09:00:00Z",
  session_id: "sess_9f2a1c",
  started_at: "2026-10-07T09:00:00Z",
  ended_at: "2026-10-07T09:00:42Z",
};

export function makeDetailRun(overrides: Partial<AutomationRun> = {}): AutomationRun {
  return { ...runBase, ...overrides };
}

/** morning-digest: completed, missed, failed (no retries), completed by Run now. */
export const morningDigestRuns: AutomationRun[] = [
  makeDetailRun(),
  makeDetailRun({
    id: "run_missed",
    status: "canceled",
    session_id: undefined,
    metadata: { reason: "misfire_grace_exceeded" },
    started_at: "2026-10-06T09:00:00Z",
    ended_at: undefined,
  }),
  makeDetailRun({
    id: "run_failed",
    status: "failed",
    attempt: 2,
    session_id: undefined,
    error: "Agent summarizer was not available",
    started_at: "2026-10-05T09:00:00Z",
    ended_at: "2026-10-05T09:00:00Z",
  }),
  makeDetailRun({
    id: "run_manual",
    fire_id: undefined,
    scheduled_at: undefined,
    session_id: "sess_4b80d2",
    started_at: "2026-10-01T14:12:00Z",
    ended_at: "2026-10-01T14:12:51Z",
  }),
];

/** rerun-delivery: handed off to the Loop, then a completed Loop run. */
export const rerunDeliveryRuns: AutomationRun[] = [
  makeDetailRun({
    id: "run_handed_off",
    status: "delegated",
    job_id: undefined,
    trigger_id: "rerun-delivery",
    fire_id: undefined,
    scheduled_at: undefined,
    session_id: undefined,
    loop_run_id: "looprun_8f3a2bce41d07a55",
    started_at: "2026-10-07T13:41:00Z",
    ended_at: undefined,
  }),
  makeDetailRun({
    id: "run_loop_done",
    job_id: undefined,
    trigger_id: "rerun-delivery",
    fire_id: undefined,
    scheduled_at: undefined,
    session_id: undefined,
    loop_run_id: "looprun_1d07a5",
    started_at: "2026-10-04T18:02:00Z",
    ended_at: "2026-10-04T18:08:12Z",
  }),
];
