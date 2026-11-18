import { createStore } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import type { FileViewerTarget } from "../lib/file-viewer-target";

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
    /**
     * Transient: what the last request asked the viewer to show (a line, or a path outside the
     * root). Never persisted; it holds until the window moves to another place.
     */
    targets: {} as Record<string, FileViewerTarget>,
  },
  on: {
    placed: (context, event: { workspaceId: string; place: FilesAppPlace }) => {
      const { [event.workspaceId]: _moved, ...targets } = context.targets;
      return {
        ...context,
        byWorkspace: { ...context.byWorkspace, [event.workspaceId]: event.place },
        targets,
      };
    },
    requested: (
      context,
      event: { workspaceId: string; place: FilesAppPlace; target?: FileViewerTarget }
    ) => {
      const { [event.workspaceId]: _previous, ...targets } = context.targets;
      return {
        byWorkspace: { ...context.byWorkspace, [event.workspaceId]: event.place },
        requests: {
          ...context.requests,
          [event.workspaceId]: (context.requests[event.workspaceId] ?? 0) + 1,
        },
        targets: event.target ? { ...targets, [event.workspaceId]: event.target } : targets,
      };
    },
  },
});

/** Reads one project's place, hydrating it from storage on first use. */
export function filesAppPlace(workspaceId: string): FilesAppPlace {
  const known = filesAppPlaceStore.getSnapshot().context.byWorkspace[workspaceId];
  return known ?? readStoredPlace(workspaceId);
}

/** What the last open request asked a project's Files window to show, until it moves elsewhere. */
export function filesAppRequestTarget(workspaceId: string): FileViewerTarget | null {
  return filesAppPlaceStore.getSnapshot().context.targets[workspaceId] ?? null;
}

function placeFiles(workspaceId: string, patch: Partial<FilesAppPlace>): void {
  const place = { ...filesAppPlace(workspaceId), ...patch };
  writeStoredPlace(workspaceId, place);
  filesAppPlaceStore.trigger.placed({ workspaceId, place });
}

/**
 * Points a project's Files window at `place` (a transcript path or a changed file, US-002.AC-6):
 * an open window re-roots and opens the path; a new one starts there. `target` carries what the
 * viewer shows beyond the path: its line, or a path outside the root as written.
 */
export function requestFilesAppPlace(
  workspaceId: string,
  place: FilesAppPlace,
  target?: FileViewerTarget
): void {
  writeStoredPlace(workspaceId, place);
  filesAppPlaceStore.trigger.requested({ workspaceId, place, ...(target ? { target } : {}) });
}

export interface UseFilesAppPlaceResult extends FilesAppPlace {
  /** Counts open requests from other surfaces; a change means "show this place now". */
  request: number;
  /** What the last request asked the viewer to show, until the window moves elsewhere. */
  requestTarget: FileViewerTarget | null;
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
  const requestTarget = useSelector(
    filesAppPlaceStore,
    snapshot => snapshot.context.targets[workspaceId] ?? null
  );
  const place = known ?? readStoredPlace(workspaceId);
  return {
    ...place,
    request,
    requestTarget,
    selectRoot: worktreeId => placeFiles(workspaceId, { worktreeId }),
    openPath: path => placeFiles(workspaceId, { path }),
  };
}
