/**
 * The decisions behind "Clean up history" (#1717), as pure functions over the daemon's dry run.
 *
 * Every number the dialog shows counts sessions removed: a child session and a subagent are
 * sessions too, so "Everything" is always at least as large as either of its parts.
 */
import type {
  SessionHistoryCleanupPlan,
  SessionHistoryCleanupRequest,
  SessionHistoryCleanupRun,
} from "../adapters/session-history-cleanup-api";

/**
 * What goes, as checks. `everything` deletes each picked main session with all it holds, so it
 * includes the other two; without it, `children` and `subagents` thin the sessions out and keep
 * the main sessions.
 */
export interface CleanupWhat {
  everything: boolean;
  children: boolean;
  subagents: boolean;
}

/**
 * Which threads are picked, by the day their main session was archived: older than N days,
 * archived before a chosen day, or any. Recent history stays unless the person picks Any time.
 */
export type CleanupRange = "7" | "15" | "30" | "before" | "all";

export interface CleanupOptions {
  what: CleanupWhat;
  range: CleanupRange;
  /** The chosen day for `before`; `null` until picked. */
  before: Date | null;
}

export const DEFAULT_CLEANUP_OPTIONS: CleanupOptions = {
  what: { everything: true, children: true, subagents: true },
  range: "7",
  before: null,
};

export interface CleanupProjectImpact {
  project: string;
  sessions: number;
}

export interface CleanupImpact {
  /** Main sessions deleted (Everything only). */
  sessions: number;
  /** Child sessions deleted. */
  children: number;
  /** Subagent sessions deleted. */
  subagents: number;
  /** Every session the run deletes: the one number the dialog leads with. */
  total: number;
  /** Child sessions kept because they are still in the list or in Later. */
  keptLive: number;
  /** Sessions deleted per project, busiest first; only shown across several projects. */
  projects: readonly CleanupProjectImpact[];
}

export type CleanupChoiceCounts = Record<keyof CleanupWhat, number>;

/** Above this many sessions the person types the count to confirm. */
export const TYPED_CONFIRM_THRESHOLD = 100;

export const CLEANUP_RANGE_ITEMS: readonly { value: CleanupRange; label: string }[] = [
  { value: "7", label: "7+ days ago" },
  { value: "15", label: "15+ days ago" },
  { value: "30", label: "30+ days ago" },
  { value: "before", label: "Before a date" },
  { value: "all", label: "Any time" },
];

const DAY_MS = 24 * 60 * 60 * 1000;
const RANGE_DAYS = [7, 15, 30] as const;

/** The list's scope the cleanup runs in: one workspace (or `@user`), or every one. */
export type CleanupScope = { workspaceId: string } | { allWorkspaces: true };

/** The dry-run and start body for the checks, the range, and the list's scope. */
export function cleanupRequest(
  options: CleanupOptions,
  scope: CleanupScope,
  sessionIds?: readonly string[]
): SessionHistoryCleanupRequest {
  const request: SessionHistoryCleanupRequest = {
    ...("allWorkspaces" in scope ? { all_workspaces: true } : { workspace_id: scope.workspaceId }),
    delete: options.what.everything
      ? { everything: true, children: true, subagents: true }
      : { ...options.what },
  };
  if (sessionIds && sessionIds.length > 0) request.session_ids = [...sessionIds];
  switch (options.range) {
    case "all":
      return request;
    case "before":
      return options.before
        ? { ...request, archived_before: options.before.toISOString() }
        : request;
    default:
      return { ...request, older_than_days: Number(options.range) };
  }
}

/** The run's own checks and range, so a dialog reopened mid-run shows what is being deleted. */
export function runCleanupOptions(run: SessionHistoryCleanupRun): CleanupOptions {
  const what = run.delete.everything
    ? { everything: true, children: true, subagents: true }
    : { ...run.delete };
  if (!run.archived_before) return { what, range: "all", before: null };
  const before = new Date(run.archived_before);
  const days = Math.round((new Date(run.started_at).getTime() - before.getTime()) / DAY_MS);
  const preset = RANGE_DAYS.find(value => value === days);
  return preset
    ? { what, range: String(preset) as CleanupRange, before: null }
    : { what, range: "before", before };
}

/** The dry run as the dialog's impact; projects are named, busiest first. */
export function cleanupImpactFromPlan(
  plan: SessionHistoryCleanupPlan,
  projectName: (workspaceId: string) => string
): CleanupImpact {
  return {
    sessions: plan.counts.sessions,
    children: plan.counts.children,
    subagents: plan.counts.subagents,
    total: plan.counts.total,
    keptLive: plan.kept_live,
    projects: plan.workspaces.map(item => ({
      project: projectName(item.workspace_id),
      sessions: item.total,
    })),
  };
}

export const EMPTY_CLEANUP_IMPACT: CleanupImpact = {
  sessions: 0,
  children: 0,
  subagents: 0,
  total: 0,
  keptLive: 0,
  projects: [],
};

export function nothingChecked(what: CleanupWhat): boolean {
  return !what.everything && !what.children && !what.subagents;
}

/** Whether the options can be previewed at all: something checked, and a day when one is due. */
export function cleanupPreviewable(options: CleanupOptions): boolean {
  return !nothingChecked(options.what) && (options.range !== "before" || options.before !== null);
}

export function plural(count: number, one: string, many: string): string {
  return `${count.toLocaleString()} ${count === 1 ? one : many}`;
}

const DAY_FORMAT = new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" });

export function formatCleanupDay(day: Date): string {
  return DAY_FORMAT.format(day);
}

/** The range as a sentence's tail: " more than 7 days ago", " before Nov 3", "". */
export function rangePhrase(options: CleanupOptions): string {
  switch (options.range) {
    case "all":
      return "";
    case "before":
      return options.before ? ` before ${formatCleanupDay(options.before)}` : "";
    default:
      return ` more than ${options.range} days ago`;
  }
}

/** What the range keeps, said plainly under the choice. */
export function rangeKeeps(options: CleanupOptions): string {
  switch (options.range) {
    case "all":
      return "Every archived session counts, however recent.";
    case "before":
      return options.before
        ? `Everything archived on or after ${formatCleanupDay(options.before)} stays.`
        : "Sessions archived before the day you pick.";
    default:
      return `Everything archived in the last ${options.range} days stays.`;
  }
}

/** The primary action carries the same number as the headline, so it doubles as confirmation. */
export function cleanupActionLabel(impact: Pick<CleanupImpact, "total">): string {
  return `Delete ${plural(impact.total, "session", "sessions")}`;
}

export function cleanupHeadline(impact: CleanupImpact): string {
  return `${plural(impact.total, "session", "sessions")} will be deleted`;
}

/** The parts behind the headline, what stays, and the one warning; zero parts never pad it. */
export function cleanupDetail(what: CleanupWhat, impact: CleanupImpact, kept = ""): string {
  const parts: string[] = [];
  if (impact.sessions > 0) {
    parts.push(plural(impact.sessions, "archived session", "archived sessions"));
  }
  if (impact.children > 0) parts.push(plural(impact.children, "child session", "child sessions"));
  if (impact.subagents > 0) parts.push(plural(impact.subagents, "subagent", "subagents"));
  const sentences: string[] = [];
  if (parts.length > 1) {
    sentences.push(`${parts.slice(0, -1).join(", ")} and ${parts.at(-1)}.`);
  }
  if (!what.everything) {
    sentences.push(what.children ? "The main sessions stay." : "Every session stays.");
  }
  if (kept) sentences.push(kept);
  sentences.push("Deleted sessions can’t be brought back.");
  return sentences.join(" ");
}

/** Why nothing matches, in the person's own terms. */
export function emptyReason(options: CleanupOptions): string {
  if (nothingChecked(options.what)) return "Choose what to delete.";
  if (options.range === "before" && !options.before) return "Pick a date to see what goes.";
  const range = rangePhrase(options);
  if (options.what.everything) {
    return range ? `No sessions were archived${range}.` : "Nothing is archived yet.";
  }
  const has =
    options.what.children && options.what.subagents
      ? "child sessions or subagents"
      : options.what.children
        ? "child sessions"
        : "subagents";
  return range ? `No session archived${range} has ${has}.` : `No archived session has ${has}.`;
}

/** The success toast: what went, in one sentence. */
export function deletedSentence(deleted: number, scopeName: string): string {
  return `Deleted ${plural(deleted, "session", "sessions")} from ${scopeName}.`;
}

/** The toast after a stop: what went before it stopped. */
export function stoppedSentence(run: Pick<SessionHistoryCleanupRun, "deleted" | "total">): string {
  return `Stopped after deleting ${run.deleted.toLocaleString()} of ${plural(run.total, "session", "sessions")}.`;
}

/** The typed confirmation matches when its digits spell the count, separators aside. */
export function typedMatches(value: string, total: number): boolean {
  return value.replace(/\D/g, "") === String(total);
}

/** Where the run is: choosing, deleting, or how a failed run ended. Success closes the dialog. */
export type CleanupPhase =
  | { kind: "form" }
  | { kind: "deleting"; done: number; total: number }
  | {
      kind: "partial";
      /** Sessions the run set out to delete, and how many went. */
      total: number;
      deleted: number;
      /** Every thread that kept sessions; `failed` lists the first 100 of them. */
      failedThreads: number;
      failed: readonly CleanupFailure[];
    }
  | { kind: "failed"; reason: string };

export interface CleanupFailure {
  sessionId: string;
  title: string;
  reason: string;
}

/**
 * The dialog's phase for the profile's latest run. A finished run only shows when it kept
 * sessions it meant to delete, stopped or not; otherwise it ended in a toast and the dialog
 * returns to its form.
 */
export function cleanupPhase(run: SessionHistoryCleanupRun | null | undefined): CleanupPhase {
  if (!run) return { kind: "form" };
  switch (run.state) {
    case "running":
      return { kind: "deleting", done: run.done, total: run.total };
    case "partial":
    case "stopped":
      if (run.failures.length === 0) return { kind: "form" };
      return {
        kind: "partial",
        total: run.total,
        deleted: run.deleted,
        failedThreads: Math.max(run.failed_threads, run.failures.length),
        failed: run.failures.map(failure => ({
          sessionId: failure.session_id,
          title: failure.title || failure.session_id,
          reason: failure.message,
        })),
      };
    case "failed":
      return {
        kind: "failed",
        reason: run.failures[0]?.message ?? "The sessions couldn’t be deleted.",
      };
    default:
      return { kind: "form" };
  }
}
