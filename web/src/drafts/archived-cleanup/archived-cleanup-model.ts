/**
 * The decisions behind "Clean up history" (#1717), as pure functions over the archive the dialog
 * previews. The prototype computes the preview from fixtures; the implementation reads the same
 * numbers from the daemon's dry run, so the copy below is the contract either way.
 */

/** What goes. Each choice is a strict description of what is deleted from each picked thread. */
export type CleanupWhat = "everything" | "children" | "subagents";

/** Which threads are picked, by the day their main session was archived. */
export type CleanupRange = "7" | "15" | "30" | "since" | "all";

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
  /** The chosen day for `since`; `null` until picked. */
  since: Date | null;
}

export interface CleanupProjectImpact {
  project: string;
  sessions: number;
}

export interface CleanupImpact {
  /** Main sessions deleted (`everything` only). */
  sessions: number;
  /** Child sessions deleted. */
  children: number;
  /** Subagent sessions deleted. */
  subagents: number;
  /** Every session the run deletes. */
  total: number;
  /** Child sessions kept because they are still in the list or in Later. */
  keptLive: number;
  /** Per project, busiest first, in the headline's unit; only shown across several projects. */
  projects: readonly CleanupProjectImpact[];
}

export const CLEANUP_RANGE_ITEMS: readonly { value: CleanupRange; label: string }[] = [
  { value: "7", label: "7 days" },
  { value: "15", label: "15 days" },
  { value: "30", label: "30 days" },
  { value: "since", label: "Since a date" },
  { value: "all", label: "All time" },
];

const DAY_MS = 24 * 60 * 60 * 1000;

/** Whole days between `since` and now; a future day matches nothing. */
function daysSince(since: Date, now: number): number {
  return Math.floor((now - since.getTime()) / DAY_MS);
}

function inRange(thread: ArchivedThread, options: CleanupOptions, now: number): boolean {
  switch (options.range) {
    case "all":
      return true;
    case "since":
      return options.since !== null && thread.archivedDaysAgo <= daysSince(options.since, now);
    default:
      return thread.archivedDaysAgo < Number(options.range);
  }
}

/** What one thread loses under a choice: the counts the dialog adds up. */
function threadImpact(thread: ArchivedThread, what: CleanupWhat) {
  switch (what) {
    case "everything":
      return { sessions: 1, children: thread.children, subagents: thread.subagents };
    case "children":
      // A child session goes with its own subagents; the main session's subagents stay.
      return { sessions: 0, children: thread.children, subagents: 0 };
    default:
      return { sessions: 0, children: 0, subagents: thread.subagents };
  }
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
    if (options.what !== "subagents") keptLive += thread.liveChildren;
    // Per project in the headline's unit, so the parts add up to the headline.
    const headline =
      options.what === "everything"
        ? impact.sessions
        : options.what === "children"
          ? impact.children
          : impact.subagents;
    if (headline > 0) {
      byProject.set(thread.project, (byProject.get(thread.project) ?? 0) + headline);
    }
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

/** The count each choice would delete in the chosen range, for the choice's own badge. */
export function cleanupChoiceCounts(
  threads: readonly ArchivedThread[],
  options: CleanupOptions,
  now = Date.now()
): Record<CleanupWhat, number> {
  const count = (what: CleanupWhat) => cleanupImpact(threads, { ...options, what }, now);
  return {
    everything: count("everything").sessions,
    children: count("children").children,
    subagents: count("subagents").subagents,
  };
}

export function plural(count: number, one: string, many: string): string {
  return `${count.toLocaleString()} ${count === 1 ? one : many}`;
}

const DAY_FORMAT = new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" });

export function formatCleanupDay(day: Date): string {
  return DAY_FORMAT.format(day);
}

/** The range as the end of a sentence: "in the last 7 days", "since Nov 3", "". */
export function rangePhrase(options: CleanupOptions): string {
  switch (options.range) {
    case "all":
      return "";
    case "since":
      return options.since ? ` since ${formatCleanupDay(options.since)}` : "";
    default:
      return ` in the last ${options.range} days`;
  }
}

/** The primary action names exactly what the run deletes, so it doubles as the confirmation. */
export function cleanupActionLabel(what: CleanupWhat, impact: CleanupImpact): string {
  switch (what) {
    case "everything":
      return `Delete ${plural(impact.sessions, "session", "sessions")}`;
    case "children":
      return `Delete ${plural(impact.children, "child session", "child sessions")}`;
    default:
      return `Delete ${plural(impact.subagents, "subagent", "subagents")}`;
  }
}

/** The headline of the preview: the thing the person picked, counted. */
export function cleanupHeadline(what: CleanupWhat, impact: CleanupImpact): string {
  switch (what) {
    case "everything":
      return `${plural(impact.sessions, "archived session", "archived sessions")} will be deleted`;
    case "children":
      return `${plural(impact.children, "child session", "child sessions")} will be deleted`;
    default:
      return `${plural(impact.subagents, "subagent", "subagents")} will be deleted`;
  }
}

/** What goes along with the headline, and what stays; zero parts never pad the sentence. */
export function cleanupDetail(what: CleanupWhat, impact: CleanupImpact, kept = ""): string {
  const along: string[] = [];
  if (what === "everything") {
    if (impact.children > 0) along.push(plural(impact.children, "child session", "child sessions"));
    if (impact.subagents > 0) along.push(plural(impact.subagents, "subagent", "subagents"));
  }
  const parts: string[] = [];
  if (along.length > 0) parts.push(`Along with ${along.join(" and ")} inside them.`);
  if (what === "children") parts.push("Their own subagents go with them.");
  if (kept) parts.push(kept);
  parts.push("Deleted sessions can’t be brought back.");
  return parts.join(" ");
}

/** Why nothing matches, in the person's own terms. */
export function emptyReason(options: CleanupOptions): string {
  if (options.range === "since" && !options.since) return "Pick a date to see what goes.";
  const noun =
    options.what === "everything"
      ? "No sessions were archived"
      : options.what === "children"
        ? "No archived session has child sessions"
        : "No archived session has subagents";
  const range = rangePhrase(options);
  return `${noun}${range || " yet"}.`;
}

/** The Archived list after a run: only `everything` removes rows; the other choices thin them. */
export function cleanupRemaining(
  threads: readonly ArchivedThread[],
  options: CleanupOptions,
  now = Date.now()
): ArchivedThread[] {
  if (options.what === "everything")
    return threads.filter(thread => !inRange(thread, options, now));
  return threads.map(thread => {
    if (!inRange(thread, options, now)) return thread;
    return options.what === "children" ? { ...thread, children: 0 } : { ...thread, subagents: 0 };
  });
}

/** The success toast: what went, in one sentence. */
export function deletedSentence(impact: CleanupImpact, scopeName: string): string {
  const parts: string[] = [];
  if (impact.sessions > 0) parts.push(plural(impact.sessions, "session", "sessions"));
  if (impact.children > 0) parts.push(plural(impact.children, "child session", "child sessions"));
  if (impact.subagents > 0) parts.push(plural(impact.subagents, "subagent", "subagents"));
  const list = parts.length > 1 ? `${parts.slice(0, -1).join(", ")} and ${parts.at(-1)}` : parts[0];
  return `Deleted ${list} from ${scopeName}.`;
}
