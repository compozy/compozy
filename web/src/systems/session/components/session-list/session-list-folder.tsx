import { ArrowDown, ArrowUp, ChevronRight, Folder, Pencil, Plus, Trash2 } from "@compozy/ui/icons";
import type * as React from "react";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
  Icon,
} from "@compozy/ui";

import { cn } from "@/lib/utils";
import { openContextMenuFromKeyboard } from "@/systems/os";

import type { SessionDragHandlers, SessionDropPosition } from "../../hooks/use-session-list-dnd";
import { BRANCH_RAIL } from "./session-list-branch-rail";

export interface SessionListFolderProps {
  folder: { id: string; name: string };
  /** Rows the folder shows on this page, children included; projects for a project folder. */
  count: number;
  /**
   * What the folder holds. A project folder (issue #1044) starts no sessions and
   * hangs its projects on a tree rail; a session folder indents its threads.
   */
  holds?: "sessions" | "projects";
  expanded: boolean;
  onToggle: () => void;
  onNewSession?: () => void;
  onRename: () => void;
  onDelete: () => void;
  onMoveUp?: () => void;
  onMoveDown?: () => void;
  drag?: SessionDragHandlers;
  /** The whole folder as the drop target for a dragged selection. */
  dropZone?: SessionDragHandlers;
  dropEdge?: SessionDropPosition | "into" | null;
  dragging?: boolean;
  testIdPrefix: string;
  children: React.ReactNode;
}

/**
 * One session folder: a collapsible header that is also a drop target, the
 * member threads, and a trailing row that starts a session inside the folder.
 */
export function SessionListFolder({
  folder,
  count,
  holds = "sessions",
  expanded,
  onToggle,
  onNewSession,
  onRename,
  onDelete,
  onMoveUp,
  onMoveDown,
  drag,
  dropZone,
  dropEdge = null,
  dragging = false,
  testIdPrefix,
  children,
}: SessionListFolderProps) {
  const testId = `${testIdPrefix}-folder-${folder.id}`;
  const header = (
    <div
      {...drag}
      data-testid={`${testId}-header`}
      data-drop={dropEdge ?? undefined}
      data-dragging={dragging || undefined}
      className={cn(
        "relative flex min-h-7 items-center rounded-md transition-colors",
        dropEdge === "into" && "bg-brand-tint ring-1 ring-brand ring-inset",
        dragging && "opacity-50"
      )}
    >
      {dropEdge === "before" || dropEdge === "after" ? (
        <span
          aria-hidden="true"
          className={cn(
            "pointer-events-none absolute inset-x-1 z-10 h-0.5 rounded-full bg-brand",
            dropEdge === "before" ? "-top-px" : "-bottom-px"
          )}
        />
      ) : null}
      <button
        type="button"
        aria-expanded={expanded}
        data-testid={`${testId}-toggle`}
        onClick={onToggle}
        className="flex min-w-0 flex-1 items-center gap-1.5 rounded-md px-2 py-1 text-left text-small-body text-muted-foreground transition-colors hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
      >
        <Icon
          as={ChevronRight}
          size="sm"
          className={cn("shrink-0 text-faint transition-transform", expanded && "rotate-90")}
        />
        <Icon as={Folder} size="sm" className="shrink-0 text-faint" />
        <span className="min-w-0 flex-1 truncate font-medium text-fg">{folder.name}</span>
        <span
          className="shrink-0 font-mono text-micro text-faint tabular-nums"
          data-testid={`${testId}-count`}
        >
          {count}
        </span>
      </button>
    </div>
  );
  return (
    <div {...dropZone} data-testid={testId} role="group" aria-label={folder.name}>
      <ContextMenu>
        <ContextMenuTrigger
          data-testid={`${testId}-menu`}
          render={header}
          onKeyDown={openContextMenuFromKeyboard}
        />
        <SessionListFolderMenu
          testId={testId}
          holds={holds}
          onNewSession={onNewSession}
          onRename={onRename}
          onDelete={onDelete}
          onMoveUp={onMoveUp}
          onMoveDown={onMoveDown}
        />
      </ContextMenu>
      {expanded ? (
        <SessionListFolderBody testId={testId} holds={holds} onNewSession={onNewSession}>
          {children}
        </SessionListFolderBody>
      ) : null}
    </div>
  );
}

type FolderVerbs = Pick<
  SessionListFolderProps,
  "holds" | "onNewSession" | "onRename" | "onDelete" | "onMoveUp" | "onMoveDown"
> & { testId: string };

/** The folder header's context menu: start a session (session folders), rename, move, delete. */
function SessionListFolderMenu({
  testId,
  holds,
  onNewSession,
  onRename,
  onDelete,
  onMoveUp,
  onMoveDown,
}: FolderVerbs) {
  return (
    <ContextMenuContent className="min-w-52">
      {holds === "sessions" ? (
        <ContextMenuItem
          data-testid={`${testId}-menu-new-session`}
          disabled={!onNewSession}
          onClick={onNewSession}
        >
          <Plus aria-hidden="true" className="size-3" />
          New session in folder
        </ContextMenuItem>
      ) : null}
      <ContextMenuItem data-testid={`${testId}-menu-rename`} onClick={onRename}>
        <Pencil aria-hidden="true" className="size-3" />
        Rename folder…
      </ContextMenuItem>
      {onMoveUp !== undefined || onMoveDown !== undefined ? (
        <>
          <ContextMenuItem
            data-testid={`${testId}-menu-up`}
            disabled={onMoveUp === undefined}
            onClick={onMoveUp}
          >
            <ArrowUp aria-hidden="true" className="size-3" />
            Move folder up
          </ContextMenuItem>
          <ContextMenuItem
            data-testid={`${testId}-menu-down`}
            disabled={onMoveDown === undefined}
            onClick={onMoveDown}
          >
            <ArrowDown aria-hidden="true" className="size-3" />
            Move folder down
          </ContextMenuItem>
        </>
      ) : null}
      <ContextMenuSeparator />
      <ContextMenuItem
        data-testid={`${testId}-menu-delete`}
        variant="destructive"
        onClick={onDelete}
      >
        <Trash2 aria-hidden="true" className="size-3" />
        Delete folder…
      </ContextMenuItem>
    </ContextMenuContent>
  );
}

/**
 * An open folder's members: a project folder hangs its projects on the tree
 * rail; a session folder indents its threads above a New session row.
 */
function SessionListFolderBody({
  testId,
  holds,
  onNewSession,
  children,
}: Pick<FolderVerbs, "testId" | "holds" | "onNewSession"> & { children: React.ReactNode }) {
  if (holds === "projects") {
    return <div className={cn(BRANCH_RAIL, "flex flex-col gap-0.5 pt-0.5")}>{children}</div>;
  }
  return (
    <div className="flex flex-col gap-0.5 pt-0.5 pl-2.5">
      {children}
      <button
        type="button"
        data-testid={`${testId}-new-session`}
        disabled={!onNewSession}
        onClick={onNewSession}
        className="flex items-center gap-2 rounded-md px-2 py-1 text-left text-small-body text-subtle transition-colors hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
      >
        <Icon as={Plus} size="sm" />
        New session
      </button>
    </div>
  );
}
