import { ArrowDown, ArrowUp, Check, Folder, FolderInput, FolderOutput } from "@compozy/ui/icons";
import { useRef } from "react";
import type * as React from "react";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from "@compozy/ui";

import { openContextMenuFromKeyboard } from "@/systems/os";

import type { SessionLifecycleActionHandlers } from "../hooks/use-session-lifecycle-actions";
import { useSessionRowMenuItems } from "../hooks/use-session-row-menu-items";
import type { SessionFolderPayload, SessionPayload } from "../types";

/** Folder and manual-order verbs a root session row adds to its context menu. */
export interface SessionRowLayoutMenu {
  folders: readonly SessionFolderPayload[];
  /** The folder holding the session, or null in the workspace root. */
  folderId: string | null;
  onMove: (folderId: string | null) => void;
  /** Present only under the Manual sort and when the row can move that way. */
  onMoveUp?: () => void;
  onMoveDown?: () => void;
}

export interface SessionRowContextMenuProps {
  session: SessionPayload;
  actions: SessionLifecycleActionHandlers;
  layout?: SessionRowLayoutMenu;
  /** The row element; it becomes the right-click and Shift+F10 target. */
  render: React.ReactElement<React.HTMLAttributes<HTMLElement>>;
  /** Content for an empty `render` wrapper (a board card's button and `…`). */
  children?: React.ReactNode;
  /**
   * The menu of the multi-selection this row belongs to (`SessionSelectionContextItems`),
   * which then replaces the row's own verbs. One trigger serves both menus, so the row
   * never remounts as the selection grows.
   */
  selectionMenu?: React.ReactNode;
}

/**
 * Right-click menu for one session row: the same lifecycle verbs as the row's
 * `…` menu, plus folder moves and the keyboard alternative to dragging. Inside
 * a multi-selection it offers the selection's verbs instead.
 */
export function SessionRowContextMenu({
  session,
  actions,
  layout,
  render,
  children,
  selectionMenu,
}: SessionRowContextMenuProps) {
  const items = useSessionRowMenuItems(session, actions);
  const disabled = actions.pendingAction !== null;
  const trigger = useRef<HTMLDivElement>(null);
  return (
    <ContextMenu>
      <ContextMenuTrigger
        ref={trigger}
        data-testid={`session-row-menu-${session.id}`}
        render={render}
        onKeyDown={openContextMenuFromKeyboard}
        {...(children === undefined ? {} : { children })}
      />
      <ContextMenuContent
        className="min-w-56"
        // Escape hands focus back to the row, so the list's keys (Esc clears the
        // selection) keep working; a pointer dismissal leaves focus where it lands.
        finalFocus={closeType =>
          closeType === "keyboard" ? rowFocusTarget(trigger.current) : false
        }
      >
        {selectionMenu !== undefined ? (
          selectionMenu
        ) : (
          <SessionRowMenuContent
            session={session}
            items={items}
            disabled={disabled}
            layout={layout}
          />
        )}
      </ContextMenuContent>
    </ContextMenu>
  );
}

/** The row's own focus target: the trigger when it takes focus, else its first button. */
function rowFocusTarget(trigger: HTMLElement | null): HTMLElement | null {
  if (!trigger || trigger.tabIndex >= 0) return trigger;
  return trigger.querySelector<HTMLElement>("button");
}

function SessionRowMenuContent({
  session,
  items,
  disabled,
  layout,
}: {
  session: SessionPayload;
  items: ReturnType<typeof useSessionRowMenuItems>;
  disabled: boolean;
  layout: SessionRowLayoutMenu | undefined;
}) {
  return (
    <>
      {items.map(item =>
        item.choices ? (
          <ContextMenuSub key={item.id}>
            <ContextMenuSubTrigger disabled={disabled}>
              <item.icon aria-hidden="true" className="size-3" />
              {item.label}
            </ContextMenuSubTrigger>
            <ContextMenuSubContent>
              {item.choices.map(choice => (
                <ContextMenuItem key={choice.id} onClick={choice.onSelect} disabled={disabled}>
                  {choice.label}
                </ContextMenuItem>
              ))}
            </ContextMenuSubContent>
          </ContextMenuSub>
        ) : (
          <ContextMenuItem
            key={item.id}
            data-testid={`session-row-menu-${item.id}-${session.id}`}
            disabled={disabled || item.disabled}
            onClick={item.onSelect}
            variant={item.destructive ? "destructive" : undefined}
          >
            <item.icon aria-hidden="true" className="size-3" />
            {item.label}
          </ContextMenuItem>
        )
      )}
      {layout ? <SessionRowLayoutItems session={session} layout={layout} /> : null}
    </>
  );
}

function SessionRowLayoutItems({
  session,
  layout,
}: {
  session: SessionPayload;
  layout: SessionRowLayoutMenu;
}) {
  const hasOrder = layout.onMoveUp !== undefined || layout.onMoveDown !== undefined;
  if (layout.folders.length === 0 && layout.folderId === null && !hasOrder) return null;
  return (
    <>
      <ContextMenuSeparator />
      {layout.folders.length > 0 ? (
        <ContextMenuSub>
          <ContextMenuSubTrigger data-testid={`session-row-menu-move-${session.id}`}>
            <FolderInput aria-hidden="true" className="size-3" />
            Move to folder
          </ContextMenuSubTrigger>
          <ContextMenuSubContent className="min-w-44">
            {layout.folders.map(folder => {
              const current = folder.id === layout.folderId;
              return (
                <ContextMenuItem
                  key={folder.id}
                  data-testid={`session-row-menu-move-${session.id}-${folder.id}`}
                  disabled={current}
                  onClick={() => layout.onMove(folder.id)}
                >
                  {current ? (
                    <Check aria-hidden="true" className="size-3" />
                  ) : (
                    <Folder aria-hidden="true" className="size-3" />
                  )}
                  <span className="min-w-0 truncate">{folder.name}</span>
                </ContextMenuItem>
              );
            })}
          </ContextMenuSubContent>
        </ContextMenuSub>
      ) : null}
      {layout.folderId !== null ? (
        <ContextMenuItem
          data-testid={`session-row-menu-unfolder-${session.id}`}
          onClick={() => layout.onMove(null)}
        >
          <FolderOutput aria-hidden="true" className="size-3" />
          Remove from folder
        </ContextMenuItem>
      ) : null}
      {hasOrder ? (
        <>
          <ContextMenuItem
            data-testid={`session-row-menu-up-${session.id}`}
            disabled={layout.onMoveUp === undefined}
            onClick={layout.onMoveUp}
          >
            <ArrowUp aria-hidden="true" className="size-3" />
            Move up
          </ContextMenuItem>
          <ContextMenuItem
            data-testid={`session-row-menu-down-${session.id}`}
            disabled={layout.onMoveDown === undefined}
            onClick={layout.onMoveDown}
          >
            <ArrowDown aria-hidden="true" className="size-3" />
            Move down
          </ContextMenuItem>
        </>
      ) : null}
    </>
  );
}
