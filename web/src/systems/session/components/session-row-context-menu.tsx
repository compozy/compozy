import { useRef } from "react";
import type * as React from "react";

import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from "@compozy/ui";

import { openContextMenuFromKeyboard } from "@/systems/os/lib/context-menu-keyboard";

import { SessionRowMenuItems, type SessionRowMenuProps } from "./session-row-menu-items";

export interface SessionRowContextMenuProps extends SessionRowMenuProps {
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
 * Right-click (and Shift+F10) menu for one session row or card: the same tree as its `…`
 * menu (`SessionRowMenuItems`). Inside a multi-selection it offers the selection's verbs instead.
 */
export function SessionRowContextMenu({
  render,
  children,
  selectionMenu,
  ...menu
}: SessionRowContextMenuProps) {
  const trigger = useRef<HTMLDivElement>(null);
  return (
    <ContextMenu>
      <ContextMenuTrigger
        ref={trigger}
        data-testid={`session-row-menu-${menu.session.id}`}
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
          <SessionRowMenuItems {...menu} host="context" />
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
