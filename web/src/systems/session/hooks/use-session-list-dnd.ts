import { useState, type DragEvent } from "react";
import { toast } from "@compozy/ui";

import { placeId } from "../lib/session-folders";
import { sessionAdoptRefusal } from "../lib/session-reparent";
import type { SessionPayload } from "../types";
import type { SessionListLayout } from "./use-session-folders";

export type SessionDropPosition = "before" | "after";

/** Where a dragged session lands on a row: between rows (reorder) or onto it (adopt). */
export type SessionRowDropEdge = SessionDropPosition | "into";

/**
 * Dropping a session onto this row makes it the row's child. `accepts` says
 * whether the dragged session may become one; refused sources never show it.
 */
export interface SessionRowNestTarget {
  accepts: (sourceId: string) => boolean;
  adopt: (sourceId: string) => void;
}

/** What the operator is dragging: a root session row, the selection, or a folder header. */
export type SessionDragItem =
  | { kind: "session"; id: string; folderId: string | null }
  | {
      kind: "selection";
      ids: readonly string[];
      folderIds: readonly (string | null)[];
    }
  | { kind: "folder"; id: string };

/** The selection a drag from one of its rows carries into a folder or the root. */
export interface SessionSelectionDrag {
  /** Rows whose drag carries the selection instead of the row alone. */
  selectedIds: ReadonlySet<string>;
  /** Movable root sessions in on-screen order, with the folder each sits in. */
  sessions: readonly { id: string; folderId: string | null }[];
  /** Why the selection cannot move; a drag from a selected row is refused with it. */
  blockedReason: string | null;
  /** Another bulk operation is running; selected rows do not drag meanwhile. */
  disabled: boolean;
  /** The off-screen count preview the browser shows under the pointer. */
  preview: HTMLElement | null;
  /** The pair whose layout the selection moves within; a grouped list wires only that group. */
  pairKey?: string | null;
  move: (folderId: string | null) => void;
}

/** Where the dragged item would land; drives the visible drop indicator. */
export type SessionDropIndicator =
  | { kind: "row"; targetId: string; position: SessionRowDropEdge }
  | { kind: "folder"; folderId: string; position: SessionDropPosition | "into" }
  | { kind: "root" };

/** One container's served order: a folder, or the workspace root when `folderId` is null. */
export interface SessionDropContainer {
  folderId: string | null;
  ids: readonly string[];
}

export type SessionDragHandlers = Pick<
  React.HTMLAttributes<HTMLElement>,
  "onDragStart" | "onDragEnd" | "onDragOver" | "onDragLeave" | "onDrop"
> & { draggable?: boolean };

export interface SessionListDnd {
  dragging: SessionDragItem | null;
  indicator: SessionDropIndicator | null;
  /**
   * Drag source and reorder target for one root session row. A selected row
   * drags the whole `selection` instead.
   */
  sessionRow: (
    sessionId: string,
    container: SessionDropContainer,
    selection?: SessionSelectionDrag,
    nest?: SessionRowNestTarget,
    targetOnly?: boolean
  ) => SessionDragHandlers;
  /** Drag source for folder reorder and drop target for "move into folder". */
  folderHeader: (folderId: string) => SessionDragHandlers;
  /**
   * The whole folder (header and members): where a dragged selection drops in.
   * The drop runs the current render's `selection`, so an operation that
   * started during the drag still blocks it.
   */
  folderZone: (folderId: string, selection?: SessionSelectionDrag) => SessionDragHandlers;
  /** The root section: dropping a foldered session there removes it from its folder. */
  rootZone: (selection?: SessionSelectionDrag) => SessionDragHandlers;
}

const DRAG_MIME = "application/x-compozy-session-layout";

/**
 * Native drag and drop for the workspace session list. Moving into a folder or
 * back to the root works under any sort; reordering rows needs the Manual sort,
 * because only that order is the operator's. Folder headers reorder under any
 * sort since folders always keep their manual order.
 *
 * Every drop resolves to one layout verb; the menu items expose the same verbs
 * for keyboard use. A dragged selection only moves: it drops anywhere on a
 * folder or on the root, never between rows.
 *
 * A single session dropped onto another row's middle — or anywhere on the row
 * while rows cannot reorder — becomes that row's child, when the row accepts it.
 */
export function useSessionListDnd(
  layout: SessionListLayout | undefined,
  options: { manual: boolean; reorderable: boolean }
): SessionListDnd {
  const [dragging, setDragging] = useState<SessionDragItem | null>(null);
  const [indicator, setIndicator] = useState<SessionDropIndicator | null>(null);
  const rowReorder = layout !== undefined && options.manual && options.reorderable;

  const start = (item: SessionDragItem) => (event: DragEvent<HTMLElement>) => {
    event.stopPropagation();
    const id = item.kind === "selection" ? item.ids.join(",") : item.id;
    event.dataTransfer?.setData(DRAG_MIME, id);
    event.dataTransfer?.setData("text/plain", id);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
    setDragging(item);
  };
  const end = () => {
    setDragging(null);
    setIndicator(null);
  };
  const accept = (event: DragEvent<HTMLElement>, next: SessionDropIndicator) => {
    event.preventDefault();
    event.stopPropagation();
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
    setIndicator(current => (sameIndicator(current, next) ? current : next));
  };
  const finish = (event: DragEvent<HTMLElement>, run: () => void) => {
    event.preventDefault();
    event.stopPropagation();
    run();
    end();
  };
  /** Starts a selection drag, or refuses it when the selection cannot move now. */
  const startSelection = (selection: SessionSelectionDrag) => (event: DragEvent<HTMLElement>) => {
    if (selection.disabled || selection.blockedReason !== null) {
      event.preventDefault();
      event.stopPropagation();
      if (!selection.disabled && selection.blockedReason) toast.warning(selection.blockedReason);
      return;
    }
    if (selection.preview) event.dataTransfer?.setDragImage(selection.preview, 12, 12);
    start({
      kind: "selection",
      ids: selection.sessions.map(session => session.id),
      folderIds: selection.sessions.map(session => session.folderId),
    })(event);
  };

  return {
    dragging,
    indicator,
    sessionRow: (sessionId, container, selection, nest, targetOnly = false) => {
      const edge = (event: DragEvent<HTMLElement>): SessionRowDropEdge | null => {
        if (dragging?.kind !== "session" || dragging.id === sessionId) return null;
        return rowDropZone(event, rowReorder && !targetOnly, nest?.accepts(dragging.id) ?? false);
      };
      return {
        draggable: !targetOnly,
        onDragStart: targetOnly
          ? undefined
          : selection?.selectedIds.has(sessionId)
            ? startSelection(selection)
            : start({ kind: "session", id: sessionId, folderId: container.folderId }),
        onDragEnd: end,
        onDragOver: event => {
          const position = edge(event);
          if (position !== null) accept(event, { kind: "row", targetId: sessionId, position });
        },
        onDragLeave: event => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
            setIndicator(current =>
              current?.kind === "row" && current.targetId === sessionId ? null : current
            );
          }
        },
        onDrop: event => {
          const position = edge(event);
          if (position === null || dragging?.kind !== "session") return;
          const moved = dragging.id;
          finish(event, () => {
            if (position === "into") {
              nest?.adopt(moved);
              return;
            }
            const next = placeId(container.ids, moved, sessionId, position);
            if (next) layout?.reorderSessions(container.folderId, next);
          });
        },
      };
    },
    folderHeader: folderId =>
      !layout
        ? {}
        : {
            draggable: true,
            onDragStart: start({ kind: "folder", id: folderId }),
            onDragEnd: end,
            onDragOver: event => {
              if (dragging?.kind === "session") {
                if (dragging.folderId !== folderId) {
                  accept(event, { kind: "folder", folderId, position: "into" });
                }
                return;
              }
              if (dragging?.kind === "folder" && dragging.id !== folderId) {
                accept(event, { kind: "folder", folderId, position: dropPosition(event) });
              }
            },
            onDrop: event => {
              if (dragging?.kind === "session" && dragging.folderId !== folderId) {
                const moved = dragging.id;
                finish(event, () => layout.moveSession(moved, folderId));
                return;
              }
              if (dragging?.kind === "folder" && dragging.id !== folderId) {
                const moved = dragging.id;
                const position = dropPosition(event);
                finish(event, () => {
                  const next = placeId(
                    layout.folders.map(folder => folder.id),
                    moved,
                    folderId,
                    position
                  );
                  if (next) layout.reorderFolders(next);
                });
              }
            },
          },
    folderZone: (folderId, selection) =>
      !layout
        ? {}
        : {
            onDragOver: event => {
              if (selectionDrops(dragging, selection, folderId)) {
                accept(event, { kind: "folder", folderId, position: "into" });
              }
            },
            onDragLeave: event => {
              if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                setIndicator(current =>
                  current?.kind === "folder" && current.folderId === folderId ? null : current
                );
              }
            },
            onDrop: event => {
              if (selection && selectionDrops(dragging, selection, folderId)) {
                finish(event, () => selection.move(folderId));
              }
            },
          },
    rootZone: selection =>
      !layout
        ? {}
        : {
            onDragOver: event => {
              if (
                (dragging?.kind === "session" && dragging.folderId !== null) ||
                selectionDrops(dragging, selection, null)
              ) {
                accept(event, { kind: "root" });
              }
            },
            onDragLeave: event => {
              if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                setIndicator(current => (current?.kind === "root" ? null : current));
              }
            },
            onDrop: event => {
              if (dragging?.kind === "session" && dragging.folderId !== null) {
                const moved = dragging.id;
                finish(event, () => layout.moveSession(moved, null));
                return;
              }
              if (selection && selectionDrops(dragging, selection, null)) {
                finish(event, () => selection.move(null));
              }
            },
          },
  };
}

/**
 * A dragged selection drops only while it can still move, still holds the
 * sessions it started with, and only where at least one of them would change
 * place.
 */
function selectionDrops(
  dragging: SessionDragItem | null,
  selection: SessionSelectionDrag | undefined,
  folderId: string | null
): boolean {
  if (dragging?.kind !== "selection" || selection === undefined) return false;
  const current = new Set(selection.sessions.map(session => session.id));
  return (
    !selection.disabled &&
    selection.blockedReason === null &&
    current.size === dragging.ids.length &&
    dragging.ids.every(id => current.has(id)) &&
    dragging.folderIds.some(folder => folder !== folderId)
  );
}

/**
 * The nest target of one row: a listed top-level row dropped onto `target`
 * becomes its child through `onAdopt`. Absent where the host cannot adopt.
 */
export function sessionRowNestTarget(
  roots: ReadonlyMap<string, SessionPayload>,
  target: SessionPayload,
  onAdopt: ((session: SessionPayload, parent: SessionPayload) => void) | undefined
): SessionRowNestTarget | undefined {
  if (!onAdopt) return undefined;
  return {
    accepts: sourceId => {
      const source = roots.get(sourceId);
      return source !== undefined && sessionAdoptRefusal(source, target) === null;
    },
    adopt: sourceId => {
      const source = roots.get(sourceId);
      if (source) onAdopt(source, target);
    },
  };
}

/** This row is being dragged, alone or as part of the selection. */
export function rowDragged(dragging: SessionDragItem | null, sessionId: string): boolean {
  if (dragging?.kind === "session") return dragging.id === sessionId;
  return dragging?.kind === "selection" && dragging.ids.includes(sessionId);
}

/**
 * The row zone under the pointer: the middle third nests while rows reorder,
 * the whole row nests while they cannot, and the halves otherwise reorder.
 */
function rowDropZone(
  event: DragEvent<HTMLElement>,
  reorder: boolean,
  nests: boolean
): SessionRowDropEdge | null {
  if (nests && !reorder) return "into";
  if (nests) {
    const bounds = event.currentTarget.getBoundingClientRect();
    const offset = event.clientY - bounds.top;
    if (offset > bounds.height / 3 && offset < (bounds.height * 2) / 3) return "into";
  }
  return reorder ? dropPosition(event) : null;
}

function dropPosition(event: DragEvent<HTMLElement>): SessionDropPosition {
  const bounds = event.currentTarget.getBoundingClientRect();
  return event.clientY < bounds.top + bounds.height / 2 ? "before" : "after";
}

function sameIndicator(left: SessionDropIndicator | null, right: SessionDropIndicator): boolean {
  if (left === null || left.kind !== right.kind) return false;
  if (left.kind === "row" && right.kind === "row") {
    return left.targetId === right.targetId && left.position === right.position;
  }
  if (left.kind === "folder" && right.kind === "folder") {
    return left.folderId === right.folderId && left.position === right.position;
  }
  return true;
}

/** The indicator edge for one row, when the pending drop targets it. */
export function rowDropEdge(
  indicator: SessionDropIndicator | null,
  sessionId: string
): SessionRowDropEdge | null {
  return indicator?.kind === "row" && indicator.targetId === sessionId ? indicator.position : null;
}

/** The indicator for one folder header, when the pending drop targets it. */
export function folderDropEdge(
  indicator: SessionDropIndicator | null,
  folderId: string
): SessionDropPosition | "into" | null {
  return indicator?.kind === "folder" && indicator.folderId === folderId
    ? indicator.position
    : null;
}
