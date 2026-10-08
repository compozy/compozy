import type { StateGlyphState } from "@compozy/ui";

import type {
  AutomationCatchUpPolicy,
  AutomationFireLimit,
  AutomationJob,
  AutomationRetry,
  AutomationRun,
  AutomationRunStatus,
  AutomationScope,
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

export function describeFireLimit(limit: AutomationFireLimit): string {
  return `Up to ${limit.max} ${limit.max === 1 ? "run" : "runs"} per ${humanizeFireWindow(limit.window)}`;
}

/** `1h` → `hour`, `30m` → `30 minutes`; unknown formats stay verbatim. */
export function humanizeFireWindow(window: string): string {
  const match = /^(\d+)([smhd])$/.exec(window.trim());
  if (!match) return window;
  const count = Number(match[1]);
  const unit = { d: "day", h: "hour", m: "minute", s: "second" }[match[2] as "d" | "h" | "m" | "s"];
  return count === 1 ? unit : `${count} ${unit}s`;
}

/** Rail Reliability row: `No retries` / `Up to 2, waiting longer each time`. */
export function describeRetryPlain(retry: AutomationRetry): string {
  if (retry.strategy === "none") return "No retries";
  return `Up to ${retry.max_retries}, waiting longer each time`;
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
 * running runs are in flight, handed-off runs are parked with their owner,
 * completed runs are done, failures fail and canceled runs (including durable
 * skips) read as stopped.
 */
const AUTOMATION_RUN_GLYPH = {
  scheduled: "queued",
  running: "running",
  delegated: "delegated",
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

/** Durable skip reasons the daemon records on a canceled run's `metadata.reason`. */
const AUTOMATION_SKIP_REASON_LABELS = {
  self_overlap: "Skipped",
  misfire_grace_exceeded: "Missed",
} as const;

export type AutomationSkipReason = keyof typeof AUTOMATION_SKIP_REASON_LABELS;

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
  return AUTOMATION_SKIP_REASON_LABELS[reason];
}

export function automationSourceLabel(source: AutomationJob["source"]): string {
  return { config: "From config", dynamic: "Created here", package: "From package" }[source];
}

/** Scope label: `workspace` reads as "Project" per the COPY.md surface aliases. */
export function automationScopeLabel(scope: AutomationScope): string {
  return scope === "workspace" ? "Project" : "Global";
}

/** Run label that names a durable skip: `Skipped` / `Missed`, else the status label. */
export function automationLastRunLabel(run: {
  status: AutomationRunStatus;
  skipReason?: AutomationSkipReason;
}): string {
  if (run.status === "canceled" && run.skipReason) {
    return automationSkipReasonLabel(run.skipReason);
  }
  return automationRunStatusLabel(run.status);
}

export interface AutomationLastRunMeta {
  tone: "danger" | "neutral";
  /** `fail` = alert glyph (danger); `skip` = skip glyph (subtle). */
  glyph: "fail" | "skip" | null;
  text: string;
  /** Relative-time anchor rendered after `text`; absent for skips and running runs. */
  at?: string;
}

const LAST_RUN_SKIP_TEXT = {
  self_overlap: "Last run skipped — the one before was still going",
  misfire_grace_exceeded: "Last run missed — CompozyOS was off at the start time",
} as const satisfies Record<AutomationSkipReason, string>;

const LAST_RUN_TEXT = {
  scheduled: "Last run scheduled",
  running: "Running now",
  delegated: "Last run handed off",
  completed: "Last run completed",
  failed: "Last run failed",
  canceled: "Last run canceled",
} as const satisfies Record<AutomationRunStatus, string>;

/** The one instant a last run is dated by, on every surface: its start, else its end. */
export function automationLastRunAt(
  run: { startedAt?: string; endedAt?: string } | undefined
): string | undefined {
  return run?.startedAt ?? run?.endedAt;
}

/** Row meta for an automation's last run (Business Rule 7); null when it never ran. */
export function automationLastRunMeta(
  run:
    | {
        status: AutomationRunStatus;
        startedAt?: string;
        endedAt?: string;
        skipReason?: AutomationSkipReason;
      }
    | undefined
): AutomationLastRunMeta | null {
  if (!run) return null;
  if (run.status === "canceled" && run.skipReason) {
    return { tone: "neutral", glyph: "skip", text: LAST_RUN_SKIP_TEXT[run.skipReason] };
  }
  const at = automationLastRunAt(run);
  return {
    tone: run.status === "failed" ? "danger" : "neutral",
    glyph: run.status === "failed" ? "fail" : null,
    text: LAST_RUN_TEXT[run.status],
    ...(run.status !== "running" && at ? { at } : {}),
  };
}
