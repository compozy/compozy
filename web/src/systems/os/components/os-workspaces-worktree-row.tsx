import { useId } from "react";
import { Check, Copy, EllipsisVertical, Trash2 } from "lucide-react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  cn,
} from "@compozy/ui";

import {
  contractHomePath,
  WorktreePath,
  WorktreeSignals,
  WorktreeStateChip,
  WorktreeStateDot,
  type WorktreeNestEntry,
} from "@/systems/workspace";
import { copyWorktreePath } from "@/systems/workspace/lib/copy-worktree-path";

const ROW_CLASS = cn(
  "group/wsov-row grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-0.5 rounded-md",
  "transition-colors duration-base ease-out"
);

function RowDot({ entry }: { entry: WorktreeNestEntry }) {
  if (entry.displayState === "ready") {
    return (
      <span
        aria-hidden="true"
        data-slot="worktree-state-dot"
        data-state="ready"
        className="inline-block size-status-dot shrink-0 rounded-full bg-success"
      />
    );
  }
  return <WorktreeStateDot state={entry.displayState} />;
}

function RowTrailing({ entry, scoped }: { entry: WorktreeNestEntry; scoped: boolean }) {
  const worktree = entry.worktree;
  return (
    <>
      {entry.inertReason ? (
        // Functional reason in its own lane — never hover-only.
        <span data-slot="os-worktree-row-reason" className="font-mono text-micro text-faint">
          {entry.inertReason}
        </span>
      ) : entry.adoptable ? (
        <span
          className={cn(
            "text-mono-id font-semibold text-info opacity-0 transition-opacity duration-base",
            "group-hover/wsov-row:opacity-100 group-focus-within/wsov-row:opacity-100",
            "group-data-[on=true]/wsov-row:opacity-100"
          )}
        >
          Adopt
        </span>
      ) : (
        <WorktreeSignals
          density="nest"
          dirty={worktree?.dirty ?? null}
          ahead={worktree?.ahead ?? null}
          behind={worktree?.behind ?? null}
          agentActivity={worktree?.agent_activity ?? "idle"}
          origin={worktree?.origin ?? ""}
          setupState={worktree?.setup_state ?? "none"}
          setupError={worktree?.setup_error}
        />
      )}
      {scoped ? (
        // One selection marker per layer: the check lives on the menu row
        // while the parent tile carries the branch badge.
        <Check aria-hidden="true" className="size-deck-glyph shrink-0 text-fg-strong" />
      ) : null}
    </>
  );
}

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
 * Two-line worktree menu row: name, then state chip + path. One trailing
 * signal (worktree-signals priority), the scope check, and a kebab with
 * Copy path / Delete worktree… — S1 nest vocabulary at menu scale.
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
        className="grid min-h-11 min-w-0 grid-cols-[16px_minmax(0,1fr)_auto] items-center gap-2 py-1.5 pl-2 text-left outline-none select-none focus-visible:outline-none"
        onClick={inert ? undefined : onSelect}
        onKeyDown={openActionsFromKeyboard}
      >
        <span id={descriptionId} className="sr-only">
          {entry.displayState}. {contractHomePath(entry.path, userHomeDir)}.{" "}
          {entry.inertReason ?? ""}
        </span>
        <span className="mt-1 flex justify-center self-start">
          <RowDot entry={entry} />
        </span>
        <span className="flex min-w-0 flex-col gap-0.5">
          <b
            className={cn(
              "min-w-0 truncate text-small-body font-medium tracking-tight",
              entry.displayState === "missing" ? "text-muted" : "text-fg-strong"
            )}
          >
            {entry.name}
          </b>
          <span className="flex min-w-0 items-center gap-1.5">
            {entry.displayState === "ready" ? null : (
              <WorktreeStateChip className="h-3.5" size="sm" state={entry.displayState} />
            )}
            <WorktreePath
              focusable={false}
              path={entry.path}
              userHomeDir={userHomeDir}
              className="text-mono-id tabular-nums"
            />
          </span>
        </span>
        <span className="flex flex-none items-center gap-workspaces-tile-gap pr-1">
          <RowTrailing entry={entry} scoped={scoped} />
        </span>
      </div>
      <DropdownMenu open={actionsOpen} onOpenChange={onActionsOpenChange}>
        <DropdownMenuTrigger
          role="menuitem"
          aria-label={`Actions for ${entry.name}`}
          data-testid={`os-workspaces-worktree-actions-${entry.key}`}
          className={cn(
            "mr-1 grid size-button-icon-xs flex-none place-items-center rounded-sm text-faint opacity-0",
            "transition-[opacity,background-color,color] duration-base ease-out",
            "group-hover/wsov-row:opacity-100 group-focus-within/wsov-row:opacity-100",
            "group-data-[on=true]/wsov-row:opacity-100 aria-expanded:opacity-100",
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
