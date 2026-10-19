import type { SessionFolderPayload, SessionPayload } from "../types";
import { isSubagentSession } from "./session-hierarchy";
import type { SessionListScope } from "./session-list-preferences";
import {
  buildSessionListThreads,
  threadVisibleOrder,
  type SessionThreadModel,
} from "./session-list-threads";

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
 * While filtering, folders without a matching thread disappear. The archive view
 * shows only folders that hold an archived session, so an empty folder never
 * reads as archived content.
 */
export function buildSessionListSections(
  threads: readonly SessionThreadModel[],
  folders: readonly SessionFolderPayload[],
  options: { filtering: boolean; archived: boolean }
): SessionListSections {
  const byFolder = new Map<string, SessionThreadModel[]>(folders.map(folder => [folder.id, []]));
  const root: SessionThreadModel[] = [];
  for (const thread of threads) {
    const folderId = sessionFolderId(thread.session);
    const members = folderId === null ? undefined : byFolder.get(folderId);
    if (members) members.push(thread);
    else root.push(thread);
  }
  const hideEmpty = options.filtering || options.archived;
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
  collapsedFolders: ReadonlySet<string>,
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
  collapsedThreads: ReadonlySet<string>,
  collapsedFolders: ReadonlySet<string>,
  forceExpanded: boolean
): string[] {
  return threadVisibleOrder(
    sectionsVisibleThreads(sections, collapsedFolders, forceExpanded),
    collapsedThreads
  );
}

export interface VisibleSessionOrderOptions {
  scope: SessionListScope;
  collapsedThreadIds: ReadonlySet<string>;
  collapsedWorkspaceIds: ReadonlySet<string>;
  workspaceGroups: ReadonlyArray<{
    workspaceId: string;
    sessions: readonly SessionPayload[];
  }>;
  /** The workspace list's folders once loaded; without them the list renders flat. */
  folders?: readonly SessionFolderPayload[];
  profileFolderGroups?: ReadonlyArray<{
    owner: { id: string };
    folders?: readonly SessionFolderPayload[];
  }>;
  collapsedFolderIds?: ReadonlySet<string>;
  archived?: boolean;
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
  if (options.scope === "all-workspaces") {
    return options.workspaceGroups.flatMap(group =>
      options.collapsedWorkspaceIds.has(group.workspaceId) ? [] : group.sessions
    );
  }

  const threads = buildSessionListThreads(sessions, "");
  const filtering = options.filtering ?? false;
  const inFolders = (
    owned: readonly SessionThreadModel[],
    folders?: readonly SessionFolderPayload[]
  ) =>
    folders
      ? sectionsVisibleThreads(
          buildSessionListSections(owned, folders, {
            filtering,
            archived: options.archived ?? false,
          }),
          options.collapsedFolderIds ?? new Set(),
          filtering
        )
      : owned;
  const visibleThreads = options.profileFolderGroups?.length
    ? options.profileFolderGroups.flatMap(group =>
        inFolders(
          threads.filter(thread => thread.session.profile_id === group.owner.id),
          group.folders
        )
      )
    : inFolders(threads, options.folders);
  const byId = new Map(sessions.map(session => [session.id, session]));
  return threadVisibleOrder(visibleThreads, options.collapsedThreadIds).flatMap(id => {
    const session = byId.get(id);
    return session ? [session] : [];
  });
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
