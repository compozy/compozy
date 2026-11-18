/**
 * The draft's menu hosts: the same tree rendered by `MenuTreeItems` as a context menu (right-click,
 * Shift+F10) or a dropdown (`…`, the selection bar's More actions), pinned open for review.
 */
import type { ReactElement, ReactNode } from "react";
import type * as React from "react";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  MenuTreeItems,
} from "@compozy/ui";

import type { SessionMenuNode } from "./session-menu-model";

/** Right-click (and Shift+F10) on `render` opens the tree. */
export function SessionContextMenu({
  nodes,
  render,
  children,
}: {
  nodes: readonly SessionMenuNode[];
  render: ReactElement<React.HTMLAttributes<HTMLElement>>;
  children?: ReactNode;
}) {
  return (
    <ContextMenu>
      <ContextMenuTrigger render={render} {...(children === undefined ? {} : { children })} />
      <ContextMenuContent className="min-w-56">
        <MenuTreeItems host="context" nodes={nodes} />
      </ContextMenuContent>
    </ContextMenu>
  );
}

/** The `…` button's dropdown; `open` starts it expanded for a gallery state. */
export function SessionDropdownMenu({
  nodes,
  trigger,
  open = false,
  openSubs,
  align = "end",
}: {
  nodes: readonly SessionMenuNode[];
  trigger: ReactElement;
  open?: boolean;
  openSubs?: readonly string[];
  align?: "start" | "end";
}) {
  return (
    <DropdownMenu defaultOpen={open} modal={false}>
      <DropdownMenuTrigger render={trigger} />
      <DropdownMenuContent align={align} className="w-auto min-w-56">
        <MenuTreeItems
          host="dropdown"
          nodes={nodes}
          openSubs={openSubs ? new Set(openSubs) : undefined}
        />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
