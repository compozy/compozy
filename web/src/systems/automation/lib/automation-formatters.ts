import type { PillTone } from "@agh/ui";

import type {
  AutomationCatchUpPolicy,
  AutomationKind,
  AutomationFireLimit,
  AutomationJob,
  AutomationRetry,
  AutomationRun,
  AutomationRunStatus,
  AutomationSchedule,
  AutomationScope,
  AutomationScopeFilter,
  AutomationTrigger,
} from "../types";

export function formatRelativeTime(dateStr?: string | null): string {
  if (!dateStr) {
    return "Not scheduled";
  }

  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) {
    return dateStr;
  }

  const diffMs = date.getTime() - Date.now();
  const diffMinutes = Math.round(diffMs / (1000 * 60));
  const absMinutes = Math.abs(diffMinutes);

  if (absMinutes < 1) {
    return diffMinutes >= 0 ? "Now" : "Just now";
  }

  if (absMinutes < 60) {
    return diffMinutes >= 0 ? `In ${absMinutes}m` : `${absMinutes}m ago`;
  }

  const absHours = Math.round(absMinutes / 60);
  if (absHours < 24) {
    return diffMinutes >= 0 ? `In ${absHours}h` : `${absHours}h ago`;
  }

  const absDays = Math.round(absHours / 24);
  return diffMinutes >= 0 ? `In ${absDays}d` : `${absDays}d ago`;
}

export function formatDateTime(dateStr?: string | null): string {
  if (!dateStr) {
    return "Unavailable";
  }

  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) {
    return dateStr;
  }

  return date.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatDate(dateStr?: string | null): string {
  if (!dateStr) {
    return "Unavailable";
  }

  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) {
    return dateStr;
  }

  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function describeSchedule(schedule?: AutomationSchedule | null): string {
  if (!schedule) {
    return "Manual";
  }

  switch (schedule.mode) {
    case "cron":
      return schedule.expr ? `Cron ${schedule.expr}` : "Cron";
    case "every":
      return schedule.interval ? `Every ${schedule.interval}` : "Every interval";
    case "at":
      return schedule.time ? `At ${formatDateTime(schedule.time)}` : "One-shot";
    default:
      return "Manual";
  }
}

export function describeTrigger(trigger: AutomationTrigger): string {
  if (trigger.event !== "webhook") {
    return trigger.event;
  }

  if (trigger.endpoint_slug) {
    return `webhook:${trigger.endpoint_slug}`;
  }

  if (trigger.webhook_id) {
    return `webhook:${trigger.webhook_id}`;
  }

  return "webhook";
}

export function describeRetry(retry: AutomationRetry): string {
  if (retry.strategy === "none") {
    return "No retries";
  }

  return `${retry.max_retries} retries from ${retry.base_delay}`;
}

export function describeFireLimit(limit: AutomationFireLimit): string {
  return `${limit.max} fires / ${limit.window}`;
}

export function formatRunTitle(run: AutomationRun): string {
  return `${run.status.toUpperCase()} · attempt ${run.attempt}`;
}

export function formatRunDuration(run: AutomationRun): string {
  const startedAt = run.started_at ? new Date(run.started_at) : null;
  if (!startedAt || Number.isNaN(startedAt.getTime())) {
    return "Queued";
  }

  if (!run.ended_at) {
    return "Running";
  }

  const endedAt = new Date(run.ended_at);
  if (Number.isNaN(endedAt.getTime())) {
    return "Unavailable";
  }

  const diffMs = Math.max(0, endedAt.getTime() - startedAt.getTime());
  const totalSeconds = Math.round(diffMs / 1000);

  if (totalSeconds < 60) {
    return `${totalSeconds}s`;
  }

  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }

  return `${minutes}m ${seconds}s`;
}

export function formatPromptPreview(prompt: string, maxLength = 72): string {
  const normalized = prompt.replaceAll(/\s+/g, " ").trim();
  if (normalized.length <= maxLength) {
    return normalized;
  }

  return `${normalized.slice(0, maxLength - 1).trimEnd()}...`;
}

type AutomationStatusKey = AutomationRunStatus | "enabled" | "disabled";

const AUTOMATION_STATUS_TONE = {
  running: "info",
  scheduled: "warning",
  delegated: "info",
  completed: "success",
  enabled: "success",
  failed: "danger",
  canceled: "neutral",
  disabled: "neutral",
} as const satisfies Record<AutomationStatusKey, PillTone>;

export function automationStatusTone(status: AutomationStatusKey): PillTone {
  return AUTOMATION_STATUS_TONE[status] ?? "neutral";
}

const CATCH_UP_POLICY_LABELS = {
  skip_missed: "Skip missed",
  coalesce: "Coalesce",
  replay: "Replay",
  run_once_on_catchup: "Run once",
} as const satisfies Record<AutomationCatchUpPolicy, string>;

/**
 * Human label for a scheduler catch-up policy. An omitted policy is the
 * target-aware default the daemon resolves per target type — never the removed
 * legacy `skip` value.
 */
export function catchUpPolicyLabel(policy?: AutomationCatchUpPolicy): string {
  return policy ? CATCH_UP_POLICY_LABELS[policy] : "Default";
}

interface AutomationSkipReasonInfo {
  label: string;
  tone: PillTone;
  detail: string;
}

/** Durable skip reasons the daemon records on a canceled run's `metadata.reason`. */
const AUTOMATION_SKIP_REASONS = {
  self_overlap: {
    label: "Overlap",
    tone: "info",
    detail: "A previous run was still active.",
  },
  misfire_grace_exceeded: {
    label: "Grace window",
    tone: "warning",
    detail: "Missed its grace window.",
  },
} as const satisfies Record<string, AutomationSkipReasonInfo>;

export type AutomationSkipReason = keyof typeof AUTOMATION_SKIP_REASONS;

/**
 * Narrow a run's durable skip evidence to a known scheduler skip reason. The
 * daemon records these reasons only on canceled runs, so a non-canceled run is
 * never treated as a skip. Run metadata is an open map, so `metadata.reason`
 * arrives untyped and is matched defensively against the two reasons recorded.
 */
export function automationRunSkipReason(run: AutomationRun): AutomationSkipReason | null {
  if (run.status !== "canceled") {
    return null;
  }
  const reason = run.metadata?.reason;
  if (reason === "self_overlap" || reason === "misfire_grace_exceeded") {
    return reason;
  }
  return null;
}

export function automationSkipReasonLabel(reason: AutomationSkipReason): string {
  return AUTOMATION_SKIP_REASONS[reason].label;
}

export function automationSkipReasonTone(reason: AutomationSkipReason): PillTone {
  return AUTOMATION_SKIP_REASONS[reason].tone;
}

export function automationSkipReasonDetail(reason: AutomationSkipReason): string {
  return AUTOMATION_SKIP_REASONS[reason].detail;
}

export function automationSourceLabel(source: AutomationJob["source"]): string {
  return { config: "CONFIG", dynamic: "DYNAMIC", package: "PACKAGE" }[source];
}

export function automationScopeLabel(scope: AutomationScope): string {
  return scope === "workspace" ? "WORKSPACE" : "GLOBAL";
}

export function automationSourceTone(source: AutomationJob["source"]): PillTone {
  return source === "dynamic" ? "info" : "neutral";
}

export function automationScopeTone(scope: AutomationScope): PillTone {
  return scope === "workspace" ? "info" : "neutral";
}

export function formatAutomationListSummary({
  activeWorkspaceName,
  kind,
  scopeFilter,
  searchQuery,
  totalCount,
  visibleCount,
}: {
  activeWorkspaceName?: string;
  kind: AutomationKind;
  scopeFilter: AutomationScopeFilter;
  searchQuery: string;
  totalCount: number;
  visibleCount: number;
}): string {
  const totalNoun =
    kind === "jobs"
      ? totalCount === 1
        ? "job"
        : "jobs"
      : totalCount === 1
        ? "trigger"
        : "triggers";
  const trimmedQuery = searchQuery.trim();
  const count = visibleCount < totalCount ? `Showing ${visibleCount} of ${totalCount}` : totalCount;

  if (trimmedQuery !== "") {
    return `${count} ${totalNoun} matching current search`;
  }

  if (totalCount === 0) {
    return `0 ${kind} found`;
  }

  if (scopeFilter === "all") {
    return `${totalCount} ${totalNoun} across all scopes`;
  }

  if (scopeFilter === "global") {
    return `${count} ${totalNoun} in global scope`;
  }

  if (activeWorkspaceName) {
    return `${count} ${totalNoun} in ${activeWorkspaceName}`;
  }

  return `${count} ${totalNoun} in workspace scope`;
}
