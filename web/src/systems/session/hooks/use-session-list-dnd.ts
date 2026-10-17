import { useState, type DragEvent } from "react";

import { placeId } from "../lib/session-folders";
import type { SessionListLayout } from "./use-session-folders";

export type SessionDropPosition = "before" | "after";

/** What the operator is dragging: a root session row or a folder header. */
export type SessionDragItem =
  | { kind: "session"; id: string; folderId: string | null }
  | { kind: "folder"; id: string };

/** Where the dragged item would land; drives the visible drop indicator. */
export type SessionDropIndicator =
  | { kind: "row"; targetId: string; position: SessionDropPosition }
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
  /** Drag source and reorder target for one root session row. */
  sessionRow: (sessionId: string, container: SessionDropContainer) => SessionDragHandlers;
  /** Drag source for folder reorder and drop target for "move into folder". */
  folderHeader: (folderId: string) => SessionDragHandlers;
  /** The root section: dropping a foldered session there removes it from its folder. */
  rootZone: () => SessionDragHandlers;
}

const DRAG_MIME = "application/x-compozy-session-layout";

/**
 * Native drag and drop for the workspace session list. Moving into a folder or
 * back to the root works under any sort; reordering rows needs the Manual sort,
 * because only that order is the operator's. Folder headers reorder under any
 * sort since folders always keep their manual order.
 *
 * Every drop resolves to one layout verb; the menu items expose the same verbs
 * for keyboard use.
 */
export function useSessionListDnd(
  layout: SessionListLayout | undefined,
  options: { manual: boolean; reorderable: boolean }
): SessionListDnd | undefined {
  const [dragging, setDragging] = useState<SessionDragItem | null>(null);
  const [indicator, setIndicator] = useState<SessionDropIndicator | null>(null);
  if (!layout) return undefined;
  const rowReorder = options.manual && options.reorderable;

  const start = (item: SessionDragItem) => (event: DragEvent<HTMLElement>) => {
    event.stopPropagation();
    event.dataTransfer?.setData(DRAG_MIME, item.id);
    event.dataTransfer?.setData("text/plain", item.id);
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

  return {
    dragging,
    indicator,
    sessionRow: (sessionId, container) => ({
      draggable: true,
      onDragStart: start({ kind: "session", id: sessionId, folderId: container.folderId }),
      onDragEnd: end,
      onDragOver: event => {
        if (dragging?.kind !== "session" || dragging.id === sessionId || !rowReorder) return;
        accept(event, { kind: "row", targetId: sessionId, position: dropPosition(event) });
      },
      onDrop: event => {
        if (dragging?.kind !== "session" || !rowReorder) return;
        const moved = dragging.id;
        const position = dropPosition(event);
        finish(event, () => {
          const next = placeId(container.ids, moved, sessionId, position);
          if (next) layout.reorderSessions(container.folderId, next);
        });
      },
    }),
    folderHeader: folderId => ({
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
    }),
    rootZone: () => ({
      onDragOver: event => {
        if (dragging?.kind === "session" && dragging.folderId !== null) {
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
        }
      },
    }),
  };
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
): SessionDropPosition | null {
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
