import { Ellipsis, Plus } from "lucide-react";

import { cn, Icon, PillDot } from "@compozy/ui";

import type { WorktreeNestEntry } from "../lib/worktree-display";
import { truncateWorktreeNest, WORKTREE_NEST_VISIBLE_LIMIT } from "../lib/worktree-sort";
import { WorktreeRow } from "./worktree-row";

export interface WorktreeAggregateProps {
  runningAgents: number;
}

/** Collapsed parents keep one quiet signal. Zero renders nothing, never "0". */
export function WorktreeAggregate({ runningAgents }: WorktreeAggregateProps) {
  if (runningAgents <= 0) return null;

  return (
    <span
      data-slot="worktree-aggregate"
      className="inline-flex shrink-0 items-center gap-[5px] font-mono text-micro text-faint"
    >
      <PillDot tone="accent" pulse size="sm" />
      {runningAgents} active
    </span>
  );
}

/**
 * Trailing affordances. A missing worktree offers resolution instead of removal,
 * because there is no directory left to remove.
 */
function NestRowActions({
  entry,
  testIdPrefix,
  onRemoveWorktree,
  onResolveMissing,
  onOpenContext,
}: {
  entry: WorktreeNestEntry;
  testIdPrefix: string;
  onRemoveWorktree?: (entry: WorktreeNestEntry) => void;
  onResolveMissing?: (entry: WorktreeNestEntry) => void;
  onOpenContext?: (entry: WorktreeNestEntry) => void;
}) {
  if (entry.adoptable) {
    return (
      <span data-slot="worktree-adopt" className="ml-auto text-mono-id font-semibold text-info">
        Adopt
      </span>
    );
  }

  if (entry.displayState === "missing" && onResolveMissing) {
    return (
      <button
        type="button"
        data-testid={`${testIdPrefix}-resolve-${entry.key}`}
        className="ml-auto rounded-md px-2 py-0.5 text-mono-id font-semibold text-info hover:bg-row-hover"
        onClick={event => {
          event.stopPropagation();
          onResolveMissing(entry);
        }}
      >
        Resolve…
      </button>
    );
  }

  // A ready worktree leads to its context, where status and the assisted exit
  // live; removal stays reachable from there.
  if (entry.displayState === "ready" && onOpenContext) {
    return (
      <button
        type="button"
        data-testid={`${testIdPrefix}-context-${entry.key}`}
        className="ml-auto rounded-md px-2 py-0.5 text-mono-id font-semibold text-subtle hover:bg-row-hover hover:text-fg"
        onClick={event => {
          event.stopPropagation();
          onOpenContext(entry);
        }}
      >
        Context…
      </button>
    );
  }

  if (entry.kind === "worktree" && onRemoveWorktree) {
    return (
      <button
        type="button"
        data-testid={`${testIdPrefix}-remove-${entry.key}`}
        className="ml-auto rounded-md px-2 py-0.5 text-mono-id font-semibold text-subtle hover:bg-danger-tint hover:text-danger"
        onClick={event => {
          event.stopPropagation();
          onRemoveWorktree(entry);
        }}
      >
        Remove…
      </button>
    );
  }

  return null;
}

export interface WorktreeNestListProps {
  entries: ReadonlyArray<WorktreeNestEntry>;
  workspaceName: string;
  selectedWorktreeId?: string | null;
  /** Selecting a discovered row IS the adoption gesture (ADR-002). */
  onSelectWorktree: (entry: WorktreeNestEntry) => void;
  onCreateWorktree?: () => void;
  /** Opens the overview scoped to this workspace. */
  onShowAll?: () => void;
  /** Opens the two-step removal for an adopted worktree. */
  onRemoveWorktree?: (entry: WorktreeNestEntry) => void;
  /** Opens the resolution dialog for a worktree whose directory disappeared. */
  onResolveMissing?: (entry: WorktreeNestEntry) => void;
  /** Opens the worktree context (status + assisted exit). */
  onOpenContext?: (entry: WorktreeNestEntry) => void;
  /** Adopted-only count, used by the overflow row's label. */
  adoptedCount: number;
  limit?: number;
  testIdPrefix?: string;
  /** `menu` renders `menuitemradio` rows for a `role="menu"` host. */
  variant?: "listbox" | "menu";
}

/**
 * Renders the locked nest: pre-sorted entries, truncated at five, with the
 * overflow row carrying the adopted-only count. Keyboard order follows DOM
 * order, which follows visual order — no `tabIndex` reordering anywhere.
 */
export function WorktreeNestList({
  entries,
  workspaceName,
  selectedWorktreeId,
  onSelectWorktree,
  onCreateWorktree,
  onShowAll,
  onRemoveWorktree,
  onResolveMissing,
  onOpenContext,
  adoptedCount,
  limit = WORKTREE_NEST_VISIBLE_LIMIT,
  testIdPrefix = "worktree-nest",
  variant = "listbox",
}: WorktreeNestListProps) {
  const { visible, overflowCount } = truncateWorktreeNest(entries, limit);
  const rowRole = variant === "menu" ? "menuitemradio" : "option";

  return (
    <div
      data-slot="worktree-nest"
      role={variant === "menu" ? "group" : "listbox"}
      aria-label={`Worktrees in ${workspaceName}`}
      // 15px indent + 1px rail: nesting reads structurally without box-drawing
      // glyphs, which screen readers announce as noise.
      className="relative ml-[15px] pl-3 before:absolute before:inset-y-0.5 before:left-0 before:w-px before:bg-line-strong"
    >
      {visible.map(entry => {
        const selected = entry.worktree !== null && entry.worktree.id === selectedWorktreeId;
        return (
          <WorktreeRow
            key={entry.key}
            entry={entry}
            density="nest"
            selected={selected}
            role={rowRole}
            data-testid={`${testIdPrefix}-item-${entry.key}`}
            aria-checked={variant === "menu" ? selected : undefined}
            aria-selected={variant === "listbox" ? selected : undefined}
            aria-disabled={entry.selectable ? undefined : true}
            tabIndex={entry.selectable ? 0 : -1}
            onClick={entry.selectable ? () => onSelectWorktree(entry) : undefined}
            onKeyDown={
              entry.selectable
                ? event => {
                    if (event.key !== "Enter" && event.key !== " ") return;
                    event.preventDefault();
                    onSelectWorktree(entry);
                  }
                : undefined
            }
            trailing={
              <NestRowActions
                entry={entry}
                testIdPrefix={testIdPrefix}
                onRemoveWorktree={onRemoveWorktree}
                onResolveMissing={onResolveMissing}
                onOpenContext={onOpenContext}
              />
            }
          />
        );
      })}

      {overflowCount > 0 && onShowAll ? (
        <button
          type="button"
          data-slot="worktree-nest-overflow"
          data-testid={`${testIdPrefix}-overflow`}
          onClick={onShowAll}
          className="flex min-h-[30px] w-full items-center gap-2 rounded-md px-2.5 text-form-label text-subtle hover:bg-row-hover"
        >
          <Icon as={Ellipsis} size="sm" className="text-faint" />
          All {adoptedCount} worktrees
        </button>
      ) : null}

      {onCreateWorktree ? (
        <button
          type="button"
          data-slot="worktree-nest-create"
          data-testid={`${testIdPrefix}-create`}
          onClick={onCreateWorktree}
          className={cn(
            "flex min-h-[30px] w-full items-center gap-2 rounded-md px-2.5",
            "text-form-label text-subtle hover:bg-row-hover"
          )}
        >
          <Icon as={Plus} size="sm" className="text-faint" />
          New worktree
        </button>
      ) : null}
    </div>
  );
}
