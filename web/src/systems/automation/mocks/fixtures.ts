import type { AutomationJob, AutomationRun, AutomationTrigger } from "../types";
import { storyAgentNames, storySessionIds, storyWorkspaceIds } from "@/storybook/fintech-scenario";

export const automationJobFixtures: AutomationJob[] = [
  {
    id: "job_launch_command_digest",
    name: "launch-command-digest",
    agent_name: storyAgentNames.product,
    prompt:
      "Summarize launch blockers, fresh approvals, and the next cutover milestone for the launch room.",
    scope: "workspace",
    workspace_id: storyWorkspaceIds.hq,
    source: "dynamic",
    target_kind: "agent",
    enabled: true,
    schedule: {
      mode: "cron",
      expr: "*/10 * * * *",
    },
    retry: {
      strategy: "none",
      max_retries: 0,
      base_delay: "",
    },
    fire_limit: {
      max: 12,
      window: "1h",
    },
    next_run: "2026-04-17T18:20:00Z",
    scheduler: {
      job_id: "job_launch_command_digest",
      registered: true,
      next_run_at: "2026-04-17T18:20:00Z",
      last_run_at: "2026-04-17T18:10:01Z",
      last_scheduled_at: "2026-04-17T18:10:00Z",
      last_fire_id: "fire_launch_command_digest_001",
      catch_up_policy: "skip_missed",
      misfire_grace_seconds: 30,
      misfire_count: 0,
      updated_at: "2026-04-17T18:10:01Z",
    },
    created_at: "2026-04-17T14:00:00Z",
    updated_at: "2026-04-17T18:10:00Z",
  },
  {
    id: "job_launch_crm_release",
    name: "launch-crm-release",
    agent_name: storyAgentNames.marketing,
    prompt:
      "Release the CRM batch once the launch room confirms canary health and approved pricing copy.",
    scope: "global",
    source: "dynamic",
    target_kind: "agent",
    enabled: true,
    retry: {
      strategy: "backoff",
      max_retries: 3,
      base_delay: "5s",
    },
    fire_limit: {
      max: 3,
      window: "30m",
    },
    created_at: "2026-04-17T15:00:00Z",
    updated_at: "2026-04-17T17:58:00Z",
  },
];

export const automationTriggerFixtures: AutomationTrigger[] = [
  {
    id: "trg_support_sla_breach",
    name: "support-sla-breach",
    agent_name: storyAgentNames.support,
    prompt:
      "Investigate the support lane when launch-day SLA exceeds {{ .Data.sla_minutes }} minutes and prepare the operator response.",
    event: "support.launch.sla_breach",
    filter: {
      "data.sla_minutes": ">=4",
    },
    scope: "workspace",
    workspace_id: storyWorkspaceIds.support,
    source: "dynamic",
    target_kind: "agent",
    enabled: true,
    retry: {
      strategy: "backoff",
      max_retries: 4,
      base_delay: "5s",
    },
    fire_limit: {
      max: 12,
      window: "1h",
    },
    endpoint_slug: "support-sla-breach",
    webhook_id: "wbh_support_sla_breach",
    webhook_secret_hash: "sha256:support-sla-breach",
    webhook_secret_present: true,
    created_at: "2026-04-17T13:00:00Z",
    updated_at: "2026-04-17T17:45:00Z",
  },
  {
    id: "trg_copy_claims_review",
    name: "copy-claims-review",
    agent_name: storyAgentNames.compliance,
    prompt:
      "Review any launch copy revision that changes pricing language or merchant guarantees before publish.",
    event: "marketing.copy.updated",
    filter: {
      "data.claim_class": "=pricing",
    },
    scope: "workspace",
    workspace_id: storyWorkspaceIds.growth,
    source: "dynamic",
    target_kind: "agent",
    enabled: true,
    retry: {
      strategy: "backoff",
      max_retries: 2,
      base_delay: "10s",
    },
    fire_limit: {
      max: 8,
      window: "1h",
    },
    endpoint_slug: "copy-claims-review",
    webhook_id: "wbh_copy_claims_review",
    webhook_secret_hash: "sha256:copy-claims-review",
    webhook_secret_present: true,
    created_at: "2026-04-17T14:30:00Z",
    updated_at: "2026-04-17T17:35:00Z",
  },
];

export const automationRunFixtures: AutomationRun[] = [
  {
    id: "run_launch_command_digest_001",
    status: "completed",
    attempt: 1,
    job_id: "job_launch_command_digest",
    fire_id: "fire_launch_command_digest_001",
    session_id: storySessionIds.product,
    scheduled_at: "2026-04-17T18:10:00Z",
    started_at: "2026-04-17T18:10:00Z",
    ended_at: "2026-04-17T18:10:42Z",
  },
  {
    id: "run_support_sla_breach_002",
    status: "running",
    attempt: 1,
    trigger_id: "trg_support_sla_breach",
    session_id: storySessionIds.support,
    started_at: "2026-04-17T18:04:00Z",
  },
];

/** Canceled runs the scheduler recorded as durable skips (never dispatched). */
export const automationRunSkipFixtures: AutomationRun[] = [
  {
    id: "run_launch_command_digest_003",
    status: "canceled",
    attempt: 1,
    job_id: "job_launch_command_digest",
    fire_id: "fire_launch_command_digest_003",
    scheduled_at: "2026-04-17T18:20:00Z",
    metadata: { reason: "self_overlap" },
  },
  {
    id: "run_launch_command_digest_004",
    status: "canceled",
    attempt: 1,
    job_id: "job_launch_command_digest",
    fire_id: "fire_launch_command_digest_004",
    scheduled_at: "2026-04-17T18:30:00Z",
    metadata: { reason: "misfire_grace_exceeded" },
  },
];

export const primaryAutomationJobFixture: AutomationJob = automationJobFixtures[0];
export const primaryAutomationTriggerFixture: AutomationTrigger = automationTriggerFixtures[0];
