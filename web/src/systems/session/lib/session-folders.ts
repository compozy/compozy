import type { SessionFolderPayload, SessionPayload } from "../types";
import { isSubagentSession } from "./session-hierarchy";
import {
  buildSessionListThreads,
  threadVisibleOrder,
  type SessionThreadModel,
} from "./session-list-threads";
import type { SessionFoldSet } from "./session-sidebar-scope";

/** One folder with the provenance threads it holds on the current page. */
export interface SessionFolderSection {
  folder: SessionFolderPayload;
  threads: SessionThreadModel[];
}

/** The workspace list split into folders on top and the unfoldered root below. */
export interface SessionListSections {
  folders: SessionFolderSection[];
  root: SessionThreadModel[];
}

/**
 * Groups threads by their root session's folder, keeping the served order inside
 * each container. Children follow their parent; they are never foldered alone.
 *
 * While filtering, folders without a matching thread disappear. The archive and
 * Later views show only folders that hold one of their sessions, so an empty
 * folder never reads as archived or parked content.
 */
export function buildSessionListSections(
  threads: readonly SessionThreadModel[],
  folders: readonly SessionFolderPayload[],
  options: { filtering: boolean; shelved: boolean }
): SessionListSections {
  const byFolder = new Map<string, SessionThreadModel[]>(folders.map(folder => [folder.id, []]));
  const root: SessionThreadModel[] = [];
  for (const thread of threads) {
    const folderId = sessionFolderId(thread.session);
    const members = folderId === null ? undefined : byFolder.get(folderId);
    if (members) members.push(thread);
    else root.push(thread);
  }
  const hideEmpty = options.filtering || options.shelved;
  const sections = folders.flatMap(folder => {
    const members = byFolder.get(folder.id) ?? [];
    return hideEmpty && members.length === 0 ? [] : [{ folder, threads: members }];
  });
  return { folders: sections, root };
}

/** The folder a session sits in, or null for the workspace root. */
export function sessionFolderId(session: SessionPayload): string | null {
  const folderId = session.folder_id?.trim();
  return folderId ? folderId : null;
}

/** Only root user sessions take a place of their own; subagents follow their parent. */
export function sessionHasOwnPlacement(session: SessionPayload): boolean {
  return !isSubagentSession(session);
}

/** Threads on screen, in order: a collapsed folder hides its threads. */
export function sectionsVisibleThreads(
  sections: SessionListSections,
  collapsedFolders: SessionFoldSet,
  forceExpanded: boolean
): SessionThreadModel[] {
  return [
    ...sections.folders.flatMap(section =>
      forceExpanded || !collapsedFolders.has(section.folder.id) ? section.threads : []
    ),
    ...sections.root,
  ];
}

/** Row ids on screen in order across folders and the root. */
export function sectionsVisibleOrder(
  sections: SessionListSections,
  collapsedThreads: SessionFoldSet,
  collapsedFolders: SessionFoldSet,
  forceExpanded: boolean
): string[] {
  return threadVisibleOrder(
    sectionsVisibleThreads(sections, collapsedFolders, forceExpanded),
    collapsedThreads
  );
}

export interface VisibleSessionOrderOptions {
  collapsedThreadIds: SessionFoldSet;
  /** The flat list's folders once loaded; without them the list renders flat. */
  folders?: readonly SessionFolderPayload[];
  /**
   * A grouped list: each group's loaded page in group order, with that group's
   * folders when its own query is one (workspace, profile) pair.
   */
  groups?: ReadonlyArray<{
    key: string;
    sessions: readonly SessionPayload[];
    folders?: readonly SessionFolderPayload[];
  }>;
  collapsedGroupKeys?: SessionFoldSet;
  collapsedFolderIds?: SessionFoldSet;
  /** The list shows the archive or Later rather than the active catalog. */
  shelved?: boolean;
  /** A search is narrowing the list, which opens every folder holding a match. */
  filtering?: boolean;
}

/**
 * Projects the row order the shared session catalog renders for the served
 * sessions. Closed groups, folders, and threads are excluded because the cycle
 * shortcut must never focus a row the operator cannot currently see; a search
 * forces folders open exactly as the list does.
 */
export function visibleSessionOrder(
  sessions: readonly SessionPayload[],
  options: VisibleSessionOrderOptions
): SessionPayload[] {
  const filtering = options.filtering ?? false;
  const ordered = (
    served: readonly SessionPayload[],
    folders: readonly SessionFolderPayload[] | undefined
  ): SessionPayload[] => {
    const threads = buildSessionListThreads(served, "");
    const visibleThreads = folders
      ? sectionsVisibleThreads(
          buildSessionListSections(threads, folders, {
            filtering,
            shelved: options.shelved ?? false,
          }),
          options.collapsedFolderIds ?? new Set(),
          filtering
        )
      : threads;
    const byId = new Map(served.map(session => [session.id, session]));
    return threadVisibleOrder(visibleThreads, options.collapsedThreadIds).flatMap(id => {
      const session = byId.get(id);
      return session ? [session] : [];
    });
  };
  if (options.groups) {
    return options.groups.flatMap(group =>
      options.collapsedGroupKeys?.has(group.key) ? [] : ordered(group.sessions, group.folders)
    );
  }
  return ordered(sessions, options.folders);
}

/** Moves `id` one step up (-1) or down (+1); returns null at the edge or when absent. */
export function shiftId(ids: readonly string[], id: string, delta: -1 | 1): string[] | null {
  const index = ids.indexOf(id);
  const target = index + delta;
  if (index < 0 || target < 0 || target >= ids.length) return null;
  const next = [...ids];
  next.splice(index, 1);
  next.splice(target, 0, id);
  return next;
}

/**
 * Places `movedId` immediately before or after `targetId`. The moved id may come
 * from another container; it is removed wherever it was and inserted here.
 * Returns null when the drop would not change the order.
 */
export function placeId(
  ids: readonly string[],
  movedId: string,
  targetId: string,
  position: "before" | "after"
): string[] | null {
  if (movedId === targetId) return null;
  const without = ids.filter(id => id !== movedId);
  const targetIndex = without.indexOf(targetId);
  if (targetIndex < 0) return null;
  const next = [...without];
  next.splice(position === "before" ? targetIndex : targetIndex + 1, 0, movedId);
  return next.length === ids.length && next.every((id, index) => id === ids[index]) ? null : next;
}

/**
 * Applies an optimistic layout change to one catalog page: listed sessions take
 * the container's folder and the listed relative order; everything else keeps
 * its place.
 */
export function applySessionOrder(
  sessions: readonly SessionPayload[],
  folderId: string | null,
  orderedIds: readonly string[]
): SessionPayload[] {
  const rank = new Map(orderedIds.map((id, index) => [id, index]));
  const slots: number[] = [];
  const moved: SessionPayload[] = [];
  sessions.forEach((session, index) => {
    if (!rank.has(session.id)) return;
    slots.push(index);
    moved.push(withFolder(session, folderId));
  });
  moved.sort((left, right) => (rank.get(left.id) ?? 0) - (rank.get(right.id) ?? 0));
  const next = [...sessions];
  slots.forEach((slot, index) => {
    const session = moved[index];
    if (session) next[slot] = session;
  });
  return next;
}

/**
 * Optimistic Manual-sort patch for a bulk move: the listed sessions take the
 * folder and lead its members in the listed order, as the daemon pins them.
 * The root also leads with unplaced sessions the page cannot tell apart, so a
 * move to the root keeps each session's slot and the reread settles the order.
 */
export function applySessionsMovedFirst(
  sessions: readonly SessionPayload[],
  folderId: string | null,
  orderedIds: readonly string[]
): SessionPayload[] {
  if (folderId === null) return applySessionOrder(sessions, folderId, orderedIds);
  const listed = new Set(orderedIds);
  const byId = new Map(sessions.map(session => [session.id, session]));
  const moved = orderedIds.flatMap(id => {
    const session = byId.get(id);
    return session ? [withFolder(session, folderId)] : [];
  });
  const firstMoved = sessions.findIndex(session => listed.has(session.id));
  const rest = sessions.filter(session => !listed.has(session.id));
  const firstMember = rest.findIndex(session => sessionFolderId(session) === folderId);
  const insertAt =
    firstMember >= 0
      ? firstMember
      : sessions.slice(0, Math.max(firstMoved, 0)).filter(session => !listed.has(session.id))
          .length;
  return [...rest.slice(0, insertAt), ...moved, ...rest.slice(insertAt)];
}

/** Optimistically moves one session to the end of a folder (or the root). */
export function applySessionMove(
  sessions: readonly SessionPayload[],
  sessionId: string,
  folderId: string | null
): SessionPayload[] {
  const moving = sessions.find(session => session.id === sessionId);
  if (!moving) return [...sessions];
  const rest = sessions.filter(session => session.id !== sessionId);
  let insertAt = rest.length;
  if (folderId) {
    rest.forEach((session, index) => {
      if (sessionFolderId(session) === folderId) insertAt = index + 1;
    });
  }
  const next = [...rest];
  next.splice(insertAt, 0, withFolder(moving, folderId));
  return next;
}

/** Clears membership for every session of a deleted folder. */
export function applyFolderRemoval(
  sessions: readonly SessionPayload[],
  folderId: string
): SessionPayload[] {
  return sessions.map(session =>
    sessionFolderId(session) === folderId ? withFolder(session, null) : session
  );
}

function withFolder(session: SessionPayload, folderId: string | null): SessionPayload {
  if (folderId) return { ...session, folder_id: folderId };
  const { folder_id: _removed, ...rest } = session;
  return rest as SessionPayload;
}

/** What a selection can move into a folder. */
export interface SessionBulkMove {
  /**
   * Selected thread roots in on-screen order; selected rows a filter or a fold
   * hides follow in catalog order. Child rows follow their parent and never
   * move on their own, and neither does a subagent whose parent is gone.
   */
  sessions: SessionPayload[];
  /** Distinct owning profiles; a folder belongs to one profile's layout. */
  profileIds: string[];
}

/** Collects the movable part of a selection in the order the operator sees it. */
export function sessionBulkMove(
  catalog: readonly SessionPayload[],
  selectedIds: ReadonlySet<string>,
  visibleOrder: readonly string[]
): SessionBulkMove {
  const rank = new Map(visibleOrder.map((id, index) => [id, index]));
  const roots = new Set(buildSessionListThreads(catalog, "").map(thread => thread.session.id));
  const sessions = catalog
    .flatMap((session, index) =>
      selectedIds.has(session.id) && roots.has(session.id) && sessionHasOwnPlacement(session)
        ? [{ session, rank: rank.get(session.id) ?? visibleOrder.length + index }]
        : []
    )
    .sort((left, right) => left.rank - right.rank)
    .map(entry => entry.session);
  const profileIds = [...new Set(sessions.map(session => session.profile_id ?? ""))];
  return { sessions, profileIds };
}

/** The sessions a move into `folderId` (null: the root) would change; already-filed ones need no write. */
export function sessionBulkMoveIds(
  sessions: readonly SessionPayload[],
  folderId: string | null
): string[] {
  return sessions.flatMap(session => (sessionFolderId(session) === folderId ? [] : [session.id]));
}

/** Why the selection cannot move into a folder, or null when it can. */
export function sessionBulkMoveBlockedReason(move: SessionBulkMove): string | null {
  if (move.sessions.length === 0) return "Child sessions move with their parent.";
  if (move.profileIds.length > 1) return "Select sessions from one profile to move them.";
  return null;
}

/** "1 session" or "N sessions", for counts in menus, toasts, and the drag preview. */
export function sessionCountLabel(count: number): string {
  return `${count} ${count === 1 ? "session" : "sessions"}`;
}
