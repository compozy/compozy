import { ChevronLeft, ChevronRight, Pencil, Plus, Trash2 } from "@compozy/ui/icons";
import type { ReactElement } from "react";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@compozy/ui";

import { openContextMenuFromKeyboard } from "../lib/context-menu-keyboard";

export interface DesktopIndicatorMenuProps {
  desktopId: string;
  /** The desktop's position among every desktop. */
  index: number;
  count: number;
  disabled: boolean;
  onRename: () => void;
  onReorder: (order: number) => void;
  onClose: () => void;
  onCreate: () => void;
  /** The indicator; right-click or the platform menu keys on it open the menu. */
  children: ReactElement;
}

/**
 * The desktop indicator's menu (US-015.AC-3): Rename, Move left, Move right, Close desktop and
 * New desktop. Move left is absent on the first desktop, Move right on the last, Close desktop on
 * the last remaining one (EC-1).
 */
export function DesktopIndicatorMenu({
  desktopId,
  index,
  count,
  disabled,
  onRename,
  onReorder,
  onClose,
  onCreate,
  children,
}: DesktopIndicatorMenuProps) {
  return (
    <ContextMenu disabled={disabled}>
      <ContextMenuTrigger
        render={children}
        onKeyDown={openContextMenuFromKeyboard}
        data-testid={`desktop-indicator-menu-${desktopId}`}
      />
      <ContextMenuContent className="min-w-44">
        <ContextMenuItem onClick={onRename}>
          <Pencil aria-hidden="true" />
          Rename
        </ContextMenuItem>
        {index > 0 ? (
          <ContextMenuItem onClick={() => onReorder(index - 1)}>
            <ChevronLeft aria-hidden="true" />
            Move left
          </ContextMenuItem>
        ) : null}
        {index < count - 1 ? (
          <ContextMenuItem onClick={() => onReorder(index + 1)}>
            <ChevronRight aria-hidden="true" />
            Move right
          </ContextMenuItem>
        ) : null}
        {count > 1 ? (
          <ContextMenuItem variant="destructive" onClick={onClose}>
            <Trash2 aria-hidden="true" />
            Close desktop
          </ContextMenuItem>
        ) : null}
        <ContextMenuSeparator />
        <ContextMenuItem onClick={onCreate}>
          <Plus aria-hidden="true" />
          New desktop
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}
