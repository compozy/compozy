/**
 * The Automations board story (`docs/design/opendesign/automations/DESIGN-NOTES.md`
 * §Shared story): seven automations in one project, four scheduled and three on
 * events, each with the `last_run` the listing reads. Times are anchored to
 * module load so relative copy ("In 14h", "7h ago") matches the boards.
 */

import type { AutomationJob, AutomationRun, AutomationTrigger } from "../types";
import { storyWorkspaceIds } from "@/storybook/fintech-scenario";

const owner = {
  profile_id: "00000000000000000000000000",
  profile_name: "default",
} as const;

const HOUR = 3_600_000;
const anchor = Date.now();
const hoursFromNow = (hours: number) => new Date(anchor + hours * HOUR).toISOString();

const workspace = { scope: "workspace", workspace_id: storyWorkspaceIds.hq } as const;
const noRetry = { strategy: "none", max_retries: 0, base_delay: "" } as const;
const backoff = { strategy: "backoff", max_retries: 2, base_delay: "5s" } as const;
const created = { created_at: hoursFromNow(-24 * 14), updated_at: hoursFromNow(-30) };

export const automationStoryJobs: AutomationJob[] = [
  {
    ...owner,
    ...workspace,
    ...created,
    id: "morning-digest",
    name: "morning-digest",
    agent_name: "summarizer",
    prompt: "Summarize yesterday's sessions.",
    target_kind: "agent",
    source: "dynamic",
    enabled: true,
    schedule: { mode: "cron", expr: "0 9 * * 1-5" },
    retry: noRetry,
    fire_limit: { max: 12, window: "1h" },
    next_run: hoursFromNow(14),
    scheduler: { job_id: "morning-digest", registered: true, next_run_at: hoursFromNow(14) },
    last_run: {
      id: "run_morning_digest_118",
      status: "completed",
      started_at: hoursFromNow(-15),
      ended_at: hoursFromNow(-15),
    },
  },
  {
    ...owner,
    ...workspace,
    ...created,
    id: "nightly-delivery",
    name: "nightly-delivery",
    agent_name: "",
    prompt: "",
    target_kind: "loop",
    loop_target: {
      workspace_id: storyWorkspaceIds.hq,
      loop_name: "software-delivery",
      inputs: { target_branch: "main" },
    },
    source: "dynamic",
    enabled: true,
    schedule: { mode: "cron", expr: "0 2 * * *" },
    retry: noRetry,
    fire_limit: { max: 4, window: "1h" },
    next_run: hoursFromNow(19),
    last_run: {
      id: "run_nightly_delivery_042",
      status: "failed",
      started_at: hoursFromNow(-7),
      ended_at: hoursFromNow(-7),
    },
  },
  {
    ...owner,
    ...workspace,
    ...created,
    id: "dependency-review",
    name: "dependency-review",
    agent_name: "",
    prompt: "",
    target_kind: "agent",
    task: {
      title: "Review dependency updates",
      owner: { kind: "pool", ref: "reviewers" },
    },
    source: "dynamic",
    enabled: false,
    schedule: { mode: "cron", expr: "0 8 * * 1" },
    retry: noRetry,
    fire_limit: { max: 1, window: "1h" },
    last_run: {
      id: "run_dependency_review_013",
      status: "completed",
      started_at: hoursFromNow(-24 * 6),
      ended_at: hoursFromNow(-24 * 6),
    },
  },
  {
    ...owner,
    ...created,
    id: "release-checklist",
    name: "release-checklist",
    scope: "global",
    agent_name: "release-manager",
    prompt: "Check the release checklist.",
    target_kind: "agent",
    source: "config",
    enabled: true,
    schedule: { mode: "every", interval: "30m" },
    retry: noRetry,
    fire_limit: { max: 4, window: "1h" },
    next_run: hoursFromNow(0.2),
    last_run: {
      id: "run_release_checklist_902",
      status: "canceled",
      started_at: hoursFromNow(-0.3),
      ended_at: hoursFromNow(-0.3),
      skip_reason: "self_overlap",
    },
  },
];

export const automationStoryTriggers: AutomationTrigger[] = [
  {
    ...owner,
    ...workspace,
    ...created,
    id: "summarize-failures",
    name: "summarize-failures",
    agent_name: "summarizer",
    prompt: "Explain what went wrong.",
    event: "session.stopped",
    filter: { "data.stop_reason": "error" },
    target_kind: "agent",
    source: "dynamic",
    enabled: true,
    retry: backoff,
    fire_limit: { max: 4, window: "1h" },
    webhook_secret_present: false,
    last_run: {
      id: "run_summarize_failures_311",
      status: "completed",
      started_at: hoursFromNow(-2),
      ended_at: hoursFromNow(-2),
    },
  },
  {
    ...owner,
    ...workspace,
    ...created,
    id: "rerun-delivery",
    name: "rerun-delivery",
    agent_name: "",
    prompt: "",
    event: "session.stopped",
    filter: { "data.stop_reason": "error" },
    target_kind: "loop",
    loop_target: {
      workspace_id: storyWorkspaceIds.hq,
      loop_name: "software-delivery",
      inputs: { implementer: "code_implementer" },
      input_mapping: { slug: "data.session_name" },
    },
    source: "dynamic",
    enabled: true,
    retry: backoff,
    fire_limit: { max: 4, window: "1h" },
    webhook_secret_present: false,
    last_run: {
      id: "run_rerun_delivery_207",
      status: "delegated",
      started_at: hoursFromNow(-2),
    },
  },
  {
    ...owner,
    ...workspace,
    ...created,
    id: "deploy-webhook",
    name: "deploy-webhook",
    agent_name: "deployer",
    prompt: "Ship it.",
    event: "webhook",
    filter: { "data.action": "deploy", "data.branch": "main" },
    target_kind: "agent",
    source: "dynamic",
    enabled: true,
    retry: backoff,
    fire_limit: { max: 6, window: "1h" },
    endpoint_slug: "deploy",
    webhook_id: "wbh_abc123",
    webhook_secret_present: true,
    ingress: {
      confirmed_at: hoursFromNow(-48),
      confirmed_endpoint_generation: 1,
      endpoint_generation: 1,
      reachability: "live",
      scope_kind: "workspace",
      subject_id: "deploy-webhook",
      subject_kind: "webhook_trigger",
      url: `https://northstar.gateway.test/api/webhooks/workspaces/${storyWorkspaceIds.hq}/deploy--wbh_abc123`,
      workspace_id: storyWorkspaceIds.hq,
    },
    last_run: {
      id: "run_deploy_webhook_054",
      status: "completed",
      started_at: hoursFromNow(-26),
      ended_at: hoursFromNow(-26),
    },
  },
];

/** Each story automation's latest run, as `…/runs?limit=1` would return it. */
export const automationStoryRuns: AutomationRun[] = [
  ...automationStoryJobs.map(job => ({ job_id: job.id, item: job })),
  ...automationStoryTriggers.map(trigger => ({ trigger_id: trigger.id, item: trigger })),
].flatMap(({ item, ...ownerId }) =>
  item.last_run
    ? [
        {
          ...owner,
          ...ownerId,
          id: item.last_run.id,
          status: item.last_run.status,
          attempt: 1,
          ...(item.last_run.started_at ? { started_at: item.last_run.started_at } : {}),
          ...(item.last_run.ended_at ? { ended_at: item.last_run.ended_at } : {}),
          ...(item.last_run.skip_reason ? { metadata: { reason: item.last_run.skip_reason } } : {}),
        },
      ]
    : []
);
