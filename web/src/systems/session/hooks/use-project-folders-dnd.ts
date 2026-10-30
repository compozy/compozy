import { useState, type DragEvent } from "react";

import { placeId } from "../lib/session-folders";
import { projectGroupContainerIds, type ProjectGroupTree } from "../lib/workspace-folders";
import type { SessionDragHandlers, SessionDropPosition } from "./use-session-list-dnd";
import type { WorkspaceLayout } from "./use-workspace-layout";

/** What is being dragged among project groups: a project folder header or a project header. */
export type ProjectDragItem =
  | { kind: "folder"; id: string }
  | { kind: "project"; workspaceId: string; folderId: string | null };

/** Where the dragged item would land; drives the visible drop indicator. */
export type ProjectDropIndicator =
  | { kind: "folder"; folderId: string; position: SessionDropPosition | "into" }
  | { kind: "project"; workspaceId: string; position: SessionDropPosition }
  | { kind: "root" };

export interface ProjectFoldersDnd {
  dragging: ProjectDragItem | null;
  indicator: ProjectDropIndicator | null;
  /** Drag source for folder reorder and drop target for "move the project into this folder". */
  folderHeader: (folderId: string) => SessionDragHandlers;
  /** Drag source and reorder target for one project header in its container. */
  projectHeader: (workspaceId: string, folderId: string | null) => SessionDragHandlers;
  /** The root: dropping a foldered project there takes it out of its folder. */
  rootZone: () => SessionDragHandlers;
}

const DRAG_MIME = "application/x-compozy-project-layout";

/**
 * Native drag and drop for project folders in All projects (issue #1044).
 * Folders reorder among folders only, since folders always sit above the root
 * projects; a folder never lands on or inside a project, so project targets
 * refuse it and show no indicator. Projects reorder within a container, drop
 * into a folder through its header, and leave one through the root.
 *
 * Its own MIME and drag state keep it apart from session drags in the same list.
 */
export function useProjectFoldersDnd(
  layout: WorkspaceLayout | undefined,
  tree: ProjectGroupTree | undefined
): ProjectFoldersDnd {
  const [dragging, setDragging] = useState<ProjectDragItem | null>(null);
  const [indicator, setIndicator] = useState<ProjectDropIndicator | null>(null);

  const start = (item: ProjectDragItem) => (event: DragEvent<HTMLElement>) => {
    event.stopPropagation();
    const id = item.kind === "folder" ? item.id : item.workspaceId;
    event.dataTransfer?.setData(DRAG_MIME, id);
    event.dataTransfer?.setData("text/plain", id);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
    setDragging(item);
  };
  const end = () => {
    setDragging(null);
    setIndicator(null);
  };
  const accept = (event: DragEvent<HTMLElement>, next: ProjectDropIndicator) => {
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
  const leave =
    (owns: (current: ProjectDropIndicator) => boolean) => (event: DragEvent<HTMLElement>) => {
      if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
        setIndicator(current => (current !== null && owns(current) ? null : current));
      }
    };

  if (!layout || !tree) {
    return {
      dragging: null,
      indicator: null,
      folderHeader: () => ({}),
      projectHeader: () => ({}),
      rootZone: () => ({}),
    };
  }
  const folderIds = tree.folders.map(node => node.folder.id);

  return {
    dragging,
    indicator,
    folderHeader: folderId => {
      const target = (event: DragEvent<HTMLElement>): ProjectDropIndicator | null => {
        if (dragging?.kind === "project") {
          return dragging.folderId === folderId
            ? null
            : { kind: "folder", folderId, position: "into" };
        }
        if (dragging?.kind === "folder" && dragging.id !== folderId) {
          return { kind: "folder", folderId, position: dropPosition(event) };
        }
        return null;
      };
      return {
        draggable: true,
        onDragStart: start({ kind: "folder", id: folderId }),
        onDragEnd: end,
        onDragOver: event => {
          const next = target(event);
          if (next) accept(event, next);
        },
        onDragLeave: leave(current => current.kind === "folder" && current.folderId === folderId),
        onDrop: event => {
          const next = target(event);
          if (next?.kind !== "folder" || dragging === null) return;
          const moved = dragging;
          finish(event, () => {
            if (moved.kind === "project") {
              layout.moveWorkspace(moved.workspaceId, folderId);
              return;
            }
            if (next.position === "into") return;
            const order = placeId(folderIds, moved.id, folderId, next.position);
            if (order) layout.reorderFolders(order);
          });
        },
      };
    },
    projectHeader: (workspaceId, folderId) => {
      // Only another project lands beside a project; a folder never does.
      const position = (event: DragEvent<HTMLElement>): SessionDropPosition | null =>
        dragging?.kind === "project" && dragging.workspaceId !== workspaceId
          ? dropPosition(event)
          : null;
      return {
        draggable: true,
        onDragStart: start({ kind: "project", workspaceId, folderId }),
        onDragEnd: end,
        onDragOver: event => {
          const edge = position(event);
          if (edge) accept(event, { kind: "project", workspaceId, position: edge });
        },
        onDragLeave: leave(
          current => current.kind === "project" && current.workspaceId === workspaceId
        ),
        onDrop: event => {
          const edge = position(event);
          if (edge === null || dragging?.kind !== "project") return;
          const moved = dragging.workspaceId;
          finish(event, () => {
            const order = placeId(
              projectGroupContainerIds(tree, folderId),
              moved,
              workspaceId,
              edge
            );
            if (order) layout.reorderWorkspaces(folderId, order);
          });
        },
      };
    },
    rootZone: () => ({
      onDragOver: event => {
        if (dragging?.kind === "project" && dragging.folderId !== null) {
          accept(event, { kind: "root" });
        }
      },
      onDragLeave: leave(current => current.kind === "root"),
      onDrop: event => {
        if (dragging?.kind !== "project" || dragging.folderId === null) return;
        const moved = dragging.workspaceId;
        finish(event, () => layout.moveWorkspace(moved, null));
      },
    }),
  };
}

function dropPosition(event: DragEvent<HTMLElement>): SessionDropPosition {
  const bounds = event.currentTarget.getBoundingClientRect();
  return event.clientY < bounds.top + bounds.height / 2 ? "before" : "after";
}

function sameIndicator(left: ProjectDropIndicator | null, right: ProjectDropIndicator): boolean {
  if (left === null || left.kind !== right.kind) return false;
  if (left.kind === "folder" && right.kind === "folder") {
    return left.folderId === right.folderId && left.position === right.position;
  }
  if (left.kind === "project" && right.kind === "project") {
    return left.workspaceId === right.workspaceId && left.position === right.position;
  }
  return true;
}

/** The indicator edge for one project header, when the pending drop targets it. */
export function projectDropEdge(
  indicator: ProjectDropIndicator | null,
  workspaceId: string
): SessionDropPosition | null {
  return indicator?.kind === "project" && indicator.workspaceId === workspaceId
    ? indicator.position
    : null;
}

/** The indicator for one folder header, when the pending drop targets it. */
export function projectFolderDropEdge(
  indicator: ProjectDropIndicator | null,
  folderId: string
): SessionDropPosition | "into" | null {
  return indicator?.kind === "folder" && indicator.folderId === folderId
    ? indicator.position
    : null;
}
