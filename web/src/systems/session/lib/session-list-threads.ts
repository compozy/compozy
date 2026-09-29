import type { SessionPayload } from "../types";
import { getSessionDisplayTitle } from "./session-display-title";
import { buildSessionTree, filterThreadSessions } from "./session-hierarchy";

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

/**
 * Groups the catalog into provenance threads that survive the filter. A
 * matching descendant keeps its root's thread, so it never renders detached.
 */
export function buildSessionListThreads(
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
    threads.push({ session: root, childSessions });
  }
  return threads;
}

/** Counts every row the threads render, roots and children alike. */
export function countThreadSessions(threads: readonly SessionThreadModel[]): number {
  return threads.reduce((count, thread) => count + 1 + thread.childSessions.length, 0);
}

/** The ids of the rows on screen in order: a collapsed thread contributes only its root. */
export function threadVisibleOrder(
  threads: readonly SessionThreadModel[],
  collapsedThreads: ReadonlySet<string>
): string[] {
  return threads.flatMap(thread => [
    thread.session.id,
    ...(collapsedThreads.has(thread.session.id)
      ? []
      : thread.childSessions.map(session => session.id)),
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
