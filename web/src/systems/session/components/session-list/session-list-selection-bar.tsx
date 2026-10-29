import { Trash2, X } from "lucide-react";

import {
  Button,
  Checkbox,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  Kbd,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  TopbarOverflowIcon,
} from "@compozy/ui";

import type { SessionSelectionVerbs } from "../../hooks/use-session-list-bulk-selection";
import { SESSION_LIST_BAR_CLASS } from "../../lib/session-list-bar-class";
import { sessionCountLabel } from "../../lib/session-folders";
import { SessionSelectionMenuItems } from "./session-list-selection-menu";

interface SessionListSelectionBarProps {
  verbs: SessionSelectionVerbs;
  hiddenByFilter: number;
  allSelected: boolean;
  onSelectAll: () => void;
  onClear: () => void;
  testIdPrefix: string;
}

/** The selection owns these verbs in both session list hosts. */
export function SessionListSelectionBar({
  verbs,
  hiddenByFilter,
  allSelected,
  onSelectAll,
  onClear,
  testIdPrefix,
}: SessionListSelectionBarProps) {
  const { count, disabled, onDelete } = verbs;
  const allLabel = allSelected ? "Clear selection" : "Select all";
  const deleteLabel = `Delete ${sessionCountLabel(count)}`;
  return (
    <div
      role="toolbar"
      aria-label="Selected sessions"
      className={SESSION_LIST_BAR_CLASS}
      data-testid={`${testIdPrefix}-selection-bar`}
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <Checkbox
              className="ml-0.5"
              aria-label={allLabel}
              checked={allSelected}
              indeterminate={!allSelected}
              onCheckedChange={allSelected ? onClear : onSelectAll}
              data-testid={`${testIdPrefix}-selection-all`}
            />
          }
        />
        <TooltipContent>{allLabel}</TooltipContent>
      </Tooltip>
      <span
        className="ml-1.5 whitespace-nowrap text-small-body font-medium text-fg tabular-nums"
        data-testid={`${testIdPrefix}-selection-count`}
      >
        {count} selected
        {hiddenByFilter > 0 ? (
          <span className="ml-1 text-micro font-normal text-subtle">· {hiddenByFilter} hidden</span>
        ) : null}
      </span>
      <span className="min-w-0 flex-1" />
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={deleteLabel}
              disabled={disabled || !onDelete}
              onClick={onDelete}
              className="shrink-0 hover:bg-danger-tint hover:text-danger focus-visible:bg-danger-tint focus-visible:text-danger"
              data-testid={`${testIdPrefix}-selection-delete`}
            >
              <Trash2 className="size-3.5" />
            </Button>
          }
        />
        <TooltipContent>{deleteLabel}</TooltipContent>
      </Tooltip>
      <DropdownMenu>
        <Tooltip>
          <TooltipTrigger
            render={
              <DropdownMenuTrigger
                aria-label="More actions"
                aria-haspopup="menu"
                data-testid={`${testIdPrefix}-selection-more`}
                render={
                  <Button variant="ghost" size="icon-sm" className="shrink-0" disabled={disabled} />
                }
              >
                <TopbarOverflowIcon aria-hidden="true" />
              </DropdownMenuTrigger>
            }
          />
          <TooltipContent>More actions</TooltipContent>
        </Tooltip>
        <DropdownMenuContent align="end" className="min-w-50">
          <SessionSelectionMenuItems
            verbs={verbs}
            kind="dropdown"
            testId={`${testIdPrefix}-selection`}
          />
        </DropdownMenuContent>
      </DropdownMenu>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label="Done"
              className="shrink-0"
              onClick={onClear}
              data-testid={`${testIdPrefix}-selection-done`}
            >
              <X className="size-3.5" />
            </Button>
          }
        />
        <TooltipContent>
          Done <Kbd>esc</Kbd>
        </TooltipContent>
      </Tooltip>
    </div>
  );
}
