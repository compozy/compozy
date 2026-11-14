/**
 * The decisions behind "Clean up history" (#1717), as pure functions over the archive the dialog
 * previews. The prototype computes the preview from fixtures; the implementation reads the same
 * numbers from the daemon's dry run, so the copy below is the contract either way.
 *
 * Every number the dialog shows counts sessions removed: a child session and a subagent are
 * sessions too, so "Everything" is always at least as large as either of its parts.
 */

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

/**
 * One row of the Archived list: an archived main session, with what hangs under it. Child
 * sessions and subagents follow their main session's archive date, never their own.
 */
export interface ArchivedThread {
  id: string;
  title: string;
  project: string;
  /** Days since the main session was archived. */
  archivedDaysAgo: number;
  /** Archived child sessions under it (any depth). */
  children: number;
  /** Subagent sessions anywhere in its tree. */
  subagents: number;
  /** Child sessions still in the session list or in Later: never deleted. */
  liveChildren: number;
}

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

/** Whole days between `day` and now. */
function daysAgo(day: Date, now: number): number {
  return Math.floor((now - day.getTime()) / DAY_MS);
}

function inRange(thread: ArchivedThread, options: CleanupOptions, now: number): boolean {
  switch (options.range) {
    case "all":
      return true;
    case "before":
      return options.before !== null && thread.archivedDaysAgo > daysAgo(options.before, now);
    default:
      return thread.archivedDaysAgo >= Number(options.range);
  }
}

/** What one thread loses under the checks. */
function threadImpact(thread: ArchivedThread, what: CleanupWhat) {
  if (what.everything) {
    return { sessions: 1, children: thread.children, subagents: thread.subagents };
  }
  return {
    sessions: 0,
    // A child session goes with its own subagents; they are counted with the child.
    children: what.children ? thread.children : 0,
    subagents: what.subagents ? thread.subagents : 0,
  };
}

export function cleanupImpact(
  threads: readonly ArchivedThread[],
  options: CleanupOptions,
  now = Date.now()
): CleanupImpact {
  let sessions = 0;
  let children = 0;
  let subagents = 0;
  let keptLive = 0;
  const byProject = new Map<string, number>();
  for (const thread of threads) {
    if (!inRange(thread, options, now)) continue;
    const impact = threadImpact(thread, options.what);
    const deleted = impact.sessions + impact.children + impact.subagents;
    if (deleted === 0) continue;
    sessions += impact.sessions;
    children += impact.children;
    subagents += impact.subagents;
    if (options.what.everything || options.what.children) keptLive += thread.liveChildren;
    byProject.set(thread.project, (byProject.get(thread.project) ?? 0) + deleted);
  }
  const projects = [...byProject]
    .map(([project, count]) => ({ project, sessions: count }))
    .sort((a, b) => b.sessions - a.sessions || a.project.localeCompare(b.project));
  return {
    sessions,
    children,
    subagents,
    total: sessions + children + subagents,
    keptLive,
    projects,
  };
}

/** Each check's own count in the chosen range: sessions it alone would remove. */
export function cleanupChoiceCounts(
  threads: readonly ArchivedThread[],
  options: CleanupOptions,
  now = Date.now()
): Record<keyof CleanupWhat, number> {
  const only = (what: Partial<CleanupWhat>) =>
    cleanupImpact(
      threads,
      { ...options, what: { everything: false, children: false, subagents: false, ...what } },
      now
    ).total;
  return {
    everything: only({ everything: true }),
    children: only({ children: true }),
    subagents: only({ subagents: true }),
  };
}

export function nothingChecked(what: CleanupWhat): boolean {
  return !what.everything && !what.children && !what.subagents;
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
export function cleanupActionLabel(impact: CleanupImpact): string {
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

/** The Archived list after a run: Everything removes rows; the other checks thin them. */
export function cleanupRemaining(
  threads: readonly ArchivedThread[],
  options: CleanupOptions,
  now = Date.now()
): ArchivedThread[] {
  if (options.what.everything) return threads.filter(thread => !inRange(thread, options, now));
  return threads.map(thread => {
    if (!inRange(thread, options, now)) return thread;
    return {
      ...thread,
      children: options.what.children ? 0 : thread.children,
      subagents: options.what.subagents ? 0 : thread.subagents,
    };
  });
}

/** The success toast: what went, in one sentence. */
export function deletedSentence(impact: CleanupImpact, scopeName: string): string {
  return `Deleted ${plural(impact.total, "session", "sessions")} from ${scopeName}.`;
}

/** The typed confirmation matches when its digits spell the count, separators aside. */
export function typedMatches(value: string, total: number): boolean {
  return value.replace(/\D/g, "") === String(total);
}
