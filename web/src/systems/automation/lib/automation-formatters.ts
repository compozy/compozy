import type { PillTone, StateGlyphState } from "@compozy/ui";

import { humanCron } from "./cron-engine-presentation";

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

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** Plain-language cadence: `Every weekday at 09:00 UTC`, `Every 30 minutes`, `Once on …`. */
export function describeSchedule(schedule?: AutomationSchedule | null): string {
  if (!schedule) {
    return "Manual";
  }

  switch (schedule.mode) {
    case "cron": {
      const human = schedule.expr ? humanCron(schedule.expr) : null;
      if (!human) return "Custom schedule";
      const hasClock = /\d{2}:\d{2}|midnight/.test(human);
      return `${capitalize(human)}${hasClock ? " UTC" : ""}`;
    }
    case "every":
      return schedule.interval
        ? `Every ${humanizeFireWindow(schedule.interval)}`
        : "Repeats on an interval";
    case "at":
      return schedule.time ? `Once on ${formatDateTime(schedule.time)}` : "Runs once";
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

  return `Up to ${retry.max_retries} retries, first after ${retry.base_delay}`;
}

export function describeFireLimit(limit: AutomationFireLimit): string {
  return `Up to ${limit.max} runs per ${humanizeFireWindow(limit.window)}`;
}

/** `1h` → `hour`, `30m` → `30 minutes`; unknown formats stay verbatim. */
export function humanizeFireWindow(window: string): string {
  const match = /^(\d+)([smhd])$/.exec(window.trim());
  if (!match) return window;
  const count = Number(match[1]);
  const unit = { d: "day", h: "hour", m: "minute", s: "second" }[match[2] as "d" | "h" | "m" | "s"];
  return count === 1 ? unit : `${count} ${unit}s`;
}

function pluralizeTimes(count: number): string {
  return count === 1 ? "1 time" : `${count} times`;
}

/** Rail Reliability row: `Backoff · 2 times` / `No retries`. */
export function describeTriggerRetry(retry: AutomationRetry): string {
  if (retry.strategy === "none") return "No retries";
  return `Backoff · ${pluralizeTimes(retry.max_retries)}`;
}

/** Rail Reliability row: `4 times / hour`. */
export function describeTriggerFireLimit(limit: AutomationFireLimit): string {
  return `${pluralizeTimes(limit.max)} / ${humanizeFireWindow(limit.window)}`;
}

/** Rail Reliability summary: `Backoff · 4 / hour`. */
export function summarizeTriggerReliability(
  retry: AutomationRetry,
  limit: AutomationFireLimit
): string {
  const strategy = retry.strategy === "backoff" ? "Backoff" : "No retry";
  return `${strategy} · ${limit.max} / ${humanizeFireWindow(limit.window)}`;
}

const AUTOMATION_RUN_STATUS_LABELS = {
  running: "Running",
  scheduled: "Scheduled",
  delegated: "Handed off",
  completed: "Completed",
  failed: "Failed",
  canceled: "Canceled",
} as const satisfies Record<AutomationRunStatus, string>;

/** Sentence-case label for a job/trigger run status. */
export function automationRunStatusLabel(status: AutomationRunStatus): string {
  return AUTOMATION_RUN_STATUS_LABELS[status] ?? status;
}

export function formatRunTitle(run: AutomationRun): string {
  return `${automationRunStatusLabel(run.status)} · attempt ${run.attempt}`;
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

/**
 * Run status → canonical `StateGlyph` state: scheduled runs wait their turn,
 * running and delegated runs are in flight, completed runs are done, failures
 * fail and canceled runs (including durable skips) read as stopped.
 */
const AUTOMATION_RUN_GLYPH = {
  scheduled: "queued",
  running: "running",
  delegated: "running",
  completed: "done",
  failed: "failed",
  canceled: "stopped",
} as const satisfies Record<AutomationRunStatus, StateGlyphState>;

export function automationRunStateGlyph(status: AutomationRunStatus): StateGlyphState {
  return AUTOMATION_RUN_GLYPH[status] ?? "idle";
}

const CATCH_UP_POLICY_LABELS = {
  skip_missed: "Skip missed",
  coalesce: "Catch up once",
  replay: "Run every missed time",
  run_once_on_catchup: "Run once, then continue",
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
    label: "Skipped",
    tone: "neutral",
    detail: "Skipped because the previous run was still going.",
  },
  misfire_grace_exceeded: {
    label: "Missed",
    tone: "warning",
    detail: "Skipped because it missed its start window.",
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
  return { config: "From config", dynamic: "Created here", package: "From package" }[source];
}

/** Scope label: `workspace` reads as "Project" per the COPY.md surface aliases. */
export function automationScopeLabel(scope: AutomationScope): string {
  return scope === "workspace" ? "Project" : "Global";
}

export function automationSourceTone(_source: AutomationJob["source"]): PillTone {
  return "neutral";
}

export function automationScopeTone(_scope: AutomationScope): PillTone {
  return "neutral";
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
    return `${totalCount} ${totalNoun} across all projects`;
  }

  if (scopeFilter === "global") {
    return `${count} global ${totalNoun}`;
  }

  if (activeWorkspaceName) {
    return `${count} ${totalNoun} in ${activeWorkspaceName}`;
  }

  return `${count} ${totalNoun} in this project`;
}
