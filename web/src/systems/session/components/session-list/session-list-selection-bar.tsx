import { Archive, RotateCcw, Square, Trash2, X } from "lucide-react";

import {
  Button,
  Checkbox,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
  Kbd,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  TopbarOverflowIcon,
} from "@compozy/ui";

interface SessionListSelectionBarProps {
  count: number;
  hiddenByFilter: number;
  allSelected: boolean;
  stoppable: number;
  archivable: number;
  unarchivable: number;
  disabled: boolean;
  onSelectAll: () => void;
  onClear: () => void;
  onDelete?: () => void;
  onStop?: () => void;
  onArchive?: () => void;
  onUnarchive?: () => void;
  testIdPrefix: string;
}

/** The selection owns these verbs in both session list hosts. */
export function SessionListSelectionBar({
  count,
  hiddenByFilter,
  allSelected,
  stoppable,
  archivable,
  unarchivable,
  disabled,
  onSelectAll,
  onClear,
  onDelete,
  onStop,
  onArchive,
  onUnarchive,
  testIdPrefix,
}: SessionListSelectionBarProps) {
  const allLabel = allSelected ? "Clear selection" : "Select all";
  const deleteLabel = `Delete ${count} sessions`;
  return (
    <div
      role="toolbar"
      aria-label="Selected sessions"
      className="flex min-h-[calc(var(--height-button-default)+var(--spacing)*3)] items-center gap-1 px-3 py-1.5"
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
        className="ml-1.5 whitespace-nowrap text-small-body font-medium text-fg-strong tabular-nums"
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
          <DropdownMenuItem
            disabled={disabled || stoppable === 0 || !onStop}
            onClick={onStop}
            data-testid={`${testIdPrefix}-selection-stop`}
          >
            <Square className="size-3" />
            Stop<DropdownMenuShortcut>{stoppable} running</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={disabled || archivable === 0 || !onArchive}
            onClick={onArchive}
            data-testid={`${testIdPrefix}-selection-archive`}
          >
            <Archive className="size-3" />
            Archive<DropdownMenuShortcut>{archivable} stopped</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={disabled || unarchivable === 0 || !onUnarchive}
            onClick={onUnarchive}
            data-testid={`${testIdPrefix}-selection-unarchive`}
          >
            <RotateCcw className="size-3" />
            Unarchive<DropdownMenuShortcut>{unarchivable} archived</DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            disabled={disabled || !onDelete}
            onClick={onDelete}
            data-testid={`${testIdPrefix}-selection-delete-item`}
          >
            <Trash2 className="size-3" />
            {deleteLabel}
            <DropdownMenuShortcut>⌘⌫</DropdownMenuShortcut>
          </DropdownMenuItem>
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
