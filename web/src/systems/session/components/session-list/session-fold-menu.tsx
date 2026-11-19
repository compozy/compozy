import { useState } from "react";
import { ChevronsDownUp, ChevronsUpDown, FolderTree, type LucideIcon } from "@compozy/ui/icons";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  Icon,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
} from "@compozy/ui";

import { SESSION_MENU_CHOICE_ITEM } from "../../lib/session-menu-choice-class";
import { SESSION_FOLD_ALLS, type SessionFoldAll } from "../../lib/session-sidebar-scope";

type SessionFoldChoice = Exclude<SessionFoldAll, null>;

const FOLD_ITEMS: Record<SessionFoldChoice, { label: string; state: string; icon: LucideIcon }> = {
  groups: { label: "Expand groups", state: "Groups expanded", icon: FolderTree },
  expanded: { label: "Expand all", state: "All expanded", icon: ChevronsUpDown },
  collapsed: { label: "Collapse all", state: "All collapsed", icon: ChevronsDownUp },
};

export interface SessionFoldMenuProps {
  /** The window's last choice; `null` until the operator makes one, so nothing is marked. */
  foldAll: SessionFoldAll;
  onFoldAllChange: (foldAll: SessionFoldChoice) => void;
  /** Prefix of the trigger's and every item's test id. */
  testId: string;
  className?: string;
}

/**
 * How deep the session list opens (#1846): folders and groups only, everything including child
 * session lists, or nothing. The glyph and the tooltip name the current choice.
 */
export function SessionFoldMenu({
  foldAll,
  onFoldAllChange,
  testId,
  className,
}: SessionFoldMenuProps) {
  const current = foldAll === null ? null : FOLD_ITEMS[foldAll];
  const state = current?.state ?? "Expand or collapse";
  // Hover only: focus returns to the trigger when the menu closes, and a tooltip opened by that
  // focus would take the next Escape meant for the surrounding dialog.
  const [tooltipOpen, setTooltipOpen] = useState(false);
  return (
    <DropdownMenu>
      <Tooltip
        open={tooltipOpen}
        onOpenChange={(open, details) => {
          if (open && details.reason === "trigger-focus") return;
          setTooltipOpen(open);
        }}
      >
        <TooltipTrigger
          render={
            <DropdownMenuTrigger
              render={
                <Button
                  aria-label={state}
                  className={cn("shrink-0", className)}
                  data-testid={`${testId}-fold-trigger`}
                  size="icon-sm"
                  type="button"
                  variant="quiet"
                />
              }
            />
          }
        >
          <Icon as={current?.icon ?? ChevronsUpDown} size="sm" />
        </TooltipTrigger>
        <TooltipContent side="bottom">{state}</TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuRadioGroup
          value={foldAll}
          onValueChange={next => onFoldAllChange(next as SessionFoldChoice)}
        >
          {SESSION_FOLD_ALLS.map(choice => (
            <DropdownMenuRadioItem
              className={SESSION_MENU_CHOICE_ITEM}
              // One choice per open: picking a depth closes the menu.
              closeOnClick
              data-testid={`${testId}-fold-${choice}`}
              key={choice}
              value={choice}
            >
              <Icon as={FOLD_ITEMS[choice].icon} size="sm" />
              {FOLD_ITEMS[choice].label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
