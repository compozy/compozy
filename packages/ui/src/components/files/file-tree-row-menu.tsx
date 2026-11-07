"use client";

import type * as React from "react";

import { DropdownMenu, DropdownMenuContent } from "../dropdown-menu";

/**
 * The row context menu surface. Pierre owns when it opens (right-click, Shift+F10, the
 * ContextMenu key); this anchors a `DropdownMenu` at the row and reports dismissal back.
 * The portaled popup carries Pierre's menu-root marker so clicks inside it stay inside.
 */
export function FileTreeRowMenu({
  anchor,
  label,
  onClose,
  children,
}: {
  /** The row's box, as a virtual element; Pierre's anchor element is a zero-size slot. */
  anchor: { getBoundingClientRect: () => DOMRect };
  label: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <DropdownMenu
      open
      modal={false}
      onOpenChange={open => {
        if (!open) onClose();
      }}
    >
      <DropdownMenuContent
        anchor={anchor}
        aria-label={label}
        data-file-tree-context-menu-root="true"
        className="w-auto"
        onClick={event => event.stopPropagation()}
      >
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
