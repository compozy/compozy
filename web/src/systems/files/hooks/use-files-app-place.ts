import { createStore } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

/** Where a project's Files window left off: its root and the open file. */
export interface FilesAppPlace {
  /** `null` is the project checkout. */
  worktreeId: string | null;
  path: string | null;
}

const EMPTY_PLACE: FilesAppPlace = { worktreeId: null, path: null };

export function filesAppPlaceStorageKey(workspaceId: string): string {
  return `compozy:files:v1:${workspaceId}`;
}

function nullableString(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

function readStoredPlace(workspaceId: string): FilesAppPlace {
  if (typeof window === "undefined") return EMPTY_PLACE;
  try {
    const raw = window.localStorage.getItem(filesAppPlaceStorageKey(workspaceId));
    if (raw === null) return EMPTY_PLACE;
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null) return EMPTY_PLACE;
    const record = parsed as Record<string, unknown>;
    return { worktreeId: nullableString(record.worktreeId), path: nullableString(record.path) };
  } catch {
    return EMPTY_PLACE;
  }
}

function writeStoredPlace(workspaceId: string, place: FilesAppPlace): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(filesAppPlaceStorageKey(workspaceId), JSON.stringify(place));
  } catch {
    // Storage full or blocked: the place still holds for this app session.
  }
}

export const filesAppPlaceStore = createStore({
  context: {
    byWorkspace: {} as Record<string, FilesAppPlace>,
    /** Transient: bumps when another surface asks the window to show a place. */
    requests: {} as Record<string, number>,
  },
  on: {
    placed: (context, event: { workspaceId: string; place: FilesAppPlace }) => ({
      ...context,
      byWorkspace: { ...context.byWorkspace, [event.workspaceId]: event.place },
    }),
    requested: (context, event: { workspaceId: string; place: FilesAppPlace }) => ({
      byWorkspace: { ...context.byWorkspace, [event.workspaceId]: event.place },
      requests: {
        ...context.requests,
        [event.workspaceId]: (context.requests[event.workspaceId] ?? 0) + 1,
      },
    }),
  },
});

/** Reads one project's place, hydrating it from storage on first use. */
export function filesAppPlace(workspaceId: string): FilesAppPlace {
  const known = filesAppPlaceStore.getSnapshot().context.byWorkspace[workspaceId];
  return known ?? readStoredPlace(workspaceId);
}

function placeFiles(workspaceId: string, patch: Partial<FilesAppPlace>): void {
  const place = { ...filesAppPlace(workspaceId), ...patch };
  writeStoredPlace(workspaceId, place);
  filesAppPlaceStore.trigger.placed({ workspaceId, place });
}

/**
 * Points a project's Files window at `place` (the Review panel's "Open in Files window",
 * US-002.AC-6): an open window re-roots and opens the path; a new one starts there.
 */
export function requestFilesAppPlace(workspaceId: string, place: FilesAppPlace): void {
  writeStoredPlace(workspaceId, place);
  filesAppPlaceStore.trigger.requested({ workspaceId, place });
}

export interface UseFilesAppPlaceResult extends FilesAppPlace {
  /** Counts open requests from other surfaces; a change means "show this place now". */
  request: number;
  /** A root switch keeps the open path; the viewer reports a path the new root lacks. */
  selectRoot: (worktreeId: string | null) => void;
  openPath: (path: string) => void;
}

/** The persisted root and selected path of one project's Files window (`compozy:files:v1:<id>`). */
export function useFilesAppPlace(workspaceId: string): UseFilesAppPlaceResult {
  const known = useSelector(
    filesAppPlaceStore,
    snapshot => snapshot.context.byWorkspace[workspaceId] ?? null
  );
  const request = useSelector(
    filesAppPlaceStore,
    snapshot => snapshot.context.requests[workspaceId] ?? 0
  );
  const place = known ?? readStoredPlace(workspaceId);
  return {
    ...place,
    request,
    selectRoot: worktreeId => placeFiles(workspaceId, { worktreeId }),
    openPath: path => placeFiles(workspaceId, { path }),
  };
}
