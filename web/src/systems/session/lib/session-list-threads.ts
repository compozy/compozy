import type { SessionPayload } from "../types";
import { getSessionDisplayTitle } from "./session-display-title";
import {
  activeChildSessionsFirst,
  buildSessionTree,
  filterThreadSessions,
  isSubagentSession,
} from "./session-hierarchy";
import { isNeedsYouBadge, sessionBadgeOf } from "./session-badge";
import { isSessionWorkRunning } from "./session-running";
import type { SessionFoldSet } from "./session-sidebar-scope";

export interface SessionThreadModel {
  session: SessionPayload;
  childSessions: SessionPayload[];
}

/** A session matches a normalized (trimmed, lowercased) filter by title or agent name. */
function sessionMatchesFilter(session: SessionPayload, normalizedFilter: string): boolean {
  if (normalizedFilter === "") return true;
  return (
    getSessionDisplayTitle(session).toLocaleLowerCase().includes(normalizedFilter) ||
    session.agent_name.toLocaleLowerCase().includes(normalizedFilter)
  );
}

// The last threads built per catalog array. Query results keep their identity
// across renders that change nothing (a fold, a hover), so the threads and every
// row's props stay identical and memoized threads skip the render.
const threadCache = new WeakMap<
  readonly SessionPayload[],
  { filter: string; threads: SessionThreadModel[] }
>();

/**
 * Groups the catalog into provenance threads that survive the filter. A
 * matching descendant keeps its root's thread, so it never renders detached;
 * each thread lists its active children first.
 * The same catalog array and filter return the same threads.
 */
export function buildSessionListThreads(
  sessions: readonly SessionPayload[],
  normalizedFilter: string
): SessionThreadModel[] {
  const cached = threadCache.get(sessions);
  if (cached?.filter === normalizedFilter) return cached.threads;
  const threads = groupSessionThreads(sessions, normalizedFilter);
  threadCache.set(sessions, { filter: normalizedFilter, threads });
  return threads;
}

function groupSessionThreads(
  sessions: readonly SessionPayload[],
  normalizedFilter: string
): SessionThreadModel[] {
  const tree = buildSessionTree(sessions);
  const threads: SessionThreadModel[] = [];
  for (const root of tree.roots) {
    const childSessions = filterThreadSessions(root, tree.childrenByParent, session =>
      sessionMatchesFilter(session, normalizedFilter)
    );
    if (childSessions === null) continue;
    // Rows, keyboard cycles and selection ranges all walk this one order.
    threads.push({ session: root, childSessions: activeChildSessionsFirst(childSessions) });
  }
  return threads;
}

/** Counts every row the threads render, roots and children alike. */
export function countThreadSessions(threads: readonly SessionThreadModel[]): number {
  return threads.reduce((count, thread) => count + 1 + thread.childSessions.length, 0);
}

/** Rows whose own work, subagents included, is running: a folder's leading signal (#1926). */
export function countRunningThreadSessions(threads: readonly SessionThreadModel[]): number {
  let running = 0;
  for (const thread of threads) {
    if (isSessionWorkRunning(thread.session)) running += 1;
    for (const child of thread.childSessions) if (isSessionWorkRunning(child)) running += 1;
  }
  return running;
}

/** Rows whose own badge needs you: a collapsed folder's leading signal, outranking running. */
export function countNeedsYouThreadSessions(threads: readonly SessionThreadModel[]): number {
  let needsYou = 0;
  for (const thread of threads) {
    if (isNeedsYouBadge(sessionBadgeOf(thread.session))) needsYou += 1;
    for (const child of thread.childSessions)
      if (isNeedsYouBadge(sessionBadgeOf(child))) needsYou += 1;
  }
  return needsYou;
}

/** The ids of the rows on screen in order: a collapsed thread keeps only its root and a revealed subagent. */
export function threadVisibleOrder(
  threads: readonly SessionThreadModel[],
  collapsedThreads: SessionFoldSet
): string[] {
  return threads.flatMap(thread => [
    thread.session.id,
    // A revealed subagent session never folds away (see SessionListThread).
    ...thread.childSessions.flatMap(session =>
      !collapsedThreads.has(thread.session.id) || isSubagentSession(session) ? [session.id] : []
    ),
  ]);
}

export type SessionListShortcut = "clear" | "select-all" | "delete";

interface ShortcutKey {
  key: string;
  metaKey: boolean;
  ctrlKey: boolean;
}

/**
 * Maps a list keystroke to its selection verb: Escape clears an active
 * selection, ⌘/Ctrl+A selects every visible row, ⌘/Ctrl+Backspace deletes the
 * selection while no lifecycle action is pending.
 */
export function sessionListShortcut(
  event: ShortcutKey,
  selectionMode: boolean,
  actionPending: boolean
): SessionListShortcut | null {
  if (event.key === "Escape") return selectionMode ? "clear" : null;
  if (!event.metaKey && !event.ctrlKey) return null;
  if (event.key.toLowerCase() === "a") return "select-all";
  if (event.key === "Backspace" && selectionMode && !actionPending) return "delete";
  return null;
}
