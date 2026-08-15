import { useId } from "react";
import { Copy, EllipsisVertical, Trash2 } from "lucide-react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  cn,
} from "@compozy/ui";

import { copyWorktreePath, WorktreeNestRow, type WorktreeNestEntry } from "@/systems/workspace";

const ROW_CLASS = cn(
  "group/wtnest grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-0.5 rounded-md",
  "transition-colors duration-base ease-out"
);

export interface OsWorkspacesWorktreeRowProps {
  entry: WorktreeNestEntry;
  /** Display-contracts home-rooted paths; copy and tooltip keep the absolute path. */
  userHomeDir?: string;
  /** This row is the active worktree scope — renders the trailing check. */
  scoped: boolean;
  /** Roving focus target (menu layer). Inert rows are never focused. */
  focused: boolean;
  actionsOpen: boolean;
  onActionsOpenChange: (open: boolean) => void;
  /** Delete stays gated to adopted, ready records — the remove flow's gate. */
  onDelete?: () => void;
  registerRow: (element: HTMLElement | null) => void;
  onSelect?: () => void;
}

/**
 * Overlay chrome around the shared `WorktreeNestRow` anatomy: roving
 * menuitemradio focus plus a kebab with Copy path / Delete worktree… —
 * the same nest vocabulary as the menubar submenu, at overlay scale.
 */
export function OsWorkspacesWorktreeRow({
  entry,
  userHomeDir,
  scoped,
  focused,
  actionsOpen,
  onActionsOpenChange,
  onDelete,
  registerRow,
  onSelect,
}: OsWorkspacesWorktreeRowProps) {
  const inert = !entry.selectable;
  const descriptionId = useId();
  const openActionsFromKeyboard = (event: React.KeyboardEvent) => {
    if (event.key === "ContextMenu" || (event.key === "F10" && event.shiftKey)) {
      event.preventDefault();
      onActionsOpenChange(true);
    }
  };
  return (
    <div
      role="group"
      data-slot="os-workspaces-worktree-row"
      data-state={entry.displayState}
      data-inert={inert ? "true" : undefined}
      data-on={focused ? "true" : undefined}
      className={cn(
        ROW_CLASS,
        inert ? "cursor-default" : "hover:bg-row-hover",
        focused && "bg-row-selected"
      )}
    >
      <div
        ref={registerRow}
        role="menuitemradio"
        aria-checked={scoped}
        aria-disabled={inert || undefined}
        aria-label={entry.name}
        aria-describedby={descriptionId}
        tabIndex={focused ? 0 : -1}
        data-testid={`os-workspaces-worktree-row-${entry.key}`}
        className="grid min-h-11 min-w-0 items-center py-1.5 pl-2 text-left outline-none select-none focus-visible:outline-none"
        onClick={inert ? undefined : onSelect}
        onKeyDown={openActionsFromKeyboard}
      >
        <WorktreeNestRow
          entry={entry}
          userHomeDir={userHomeDir}
          checked={scoped}
          descriptionId={descriptionId}
        />
      </div>
      <DropdownMenu open={actionsOpen} onOpenChange={onActionsOpenChange}>
        <DropdownMenuTrigger
          role="menuitem"
          aria-label={`Actions for ${entry.name}`}
          data-testid={`os-workspaces-worktree-actions-${entry.key}`}
          className={cn(
            "mr-1 grid size-button-icon-xs flex-none place-items-center rounded-sm text-faint opacity-0",
            "transition-[opacity,background-color,color] duration-base ease-out",
            "group-hover/wtnest:opacity-100 group-focus-within/wtnest:opacity-100",
            "group-data-[on=true]/wtnest:opacity-100 aria-expanded:opacity-100",
            "hover:bg-btn-default-fill hover:text-fg-strong",
            "focus-visible:bg-btn-default-fill focus-visible:text-fg-strong focus-visible:opacity-100 focus-visible:outline-none"
          )}
        >
          <EllipsisVertical className="size-deck-glyph" />
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          side="bottom"
          className="min-w-42 rounded-lg border-line-strong bg-shell-glass-pop shadow-overlay backdrop-blur-shell-menu backdrop-saturate-[1.25]"
        >
          <DropdownMenuItem
            data-testid={`os-workspaces-worktree-copy-${entry.key}`}
            onClick={() => copyWorktreePath(entry.path)}
          >
            <Copy className="size-deck-glyph text-subtle" />
            Copy path
          </DropdownMenuItem>
          {onDelete ? (
            <DropdownMenuItem
              variant="destructive"
              data-testid={`os-workspaces-worktree-delete-${entry.key}`}
              onClick={onDelete}
            >
              <Trash2 className="size-deck-glyph" />
              Delete worktree…
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
