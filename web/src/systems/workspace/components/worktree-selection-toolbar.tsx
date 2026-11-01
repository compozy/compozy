import type { ComponentProps, ReactNode } from "react";
import { ListChecks, Trash2 } from "@compozy/ui/icons";
import { Button, MenubarItem, cn } from "@compozy/ui";
import type { WorktreeRemovalSelection } from "@/systems/workspace/hooks/use-worktree-removal-selection";

/** One-line action rows keep the nest rhythm beside two-line worktree rows. */
const FOOTER_ROW_CLASS = "min-h-control-compact";
/** Select all / Cancel sit beside the count as quiet text-weight actions. */
const COMPACT_ACTION_CLASS = "min-h-6 w-auto shrink-0 px-2 py-0.5 text-micro text-muted-foreground";

interface ToolbarAction {
  id: string;
  label: string;
  run: () => void;
  disabled: boolean;
  icon?: ReactNode;
  destructive?: boolean;
}

function worktreeRemoveSelectedLabel(count: number) {
  return `Remove ${count} ${count === 1 ? "worktree" : "worktrees"}…`;
}

/**
 * The nest's selection footer, opening with its own hairline so hosts drop
 * theirs above the create row. At rest it is one quiet "Select worktrees…" row;
 * in selection mode it becomes a count with Select all / Cancel, then one
 * destructive row that names exactly how many worktrees it removes. Activation
 * stays inside the list, so Enter and Space never reach the host navigation.
 */
export function WorktreeSelectionToolbar({
  selection,
  menu = false,
  className,
  onKeyDown,
  ...props
}: ComponentProps<"div"> & {
  selection: WorktreeRemovalSelection;
  menu?: boolean;
}) {
  if (!selection.enabled) return null;
  const render = (action: ToolbarAction, rowClassName?: string) =>
    menu ? (
      <MenubarItem
        key={action.id}
        closeOnClick={false}
        variant={action.destructive ? "destructive" : "default"}
        disabled={action.disabled}
        className={rowClassName}
        onClick={action.run}
      >
        {action.icon}
        {action.label}
      </MenubarItem>
    ) : (
      <Button
        key={action.id}
        size="sm"
        variant="ghost"
        disabled={action.disabled}
        className={cn(
          rowClassName,
          action.destructive && "text-destructive hover:bg-destructive-tint hover:text-destructive"
        )}
        onClick={action.run}
      >
        {action.icon}
        {action.label}
      </Button>
    );
  return (
    <div
      {...props}
      data-slot="worktree-selection-toolbar"
      data-mode={selection.mode ? "select" : "idle"}
      className={cn("flex flex-wrap items-center gap-y-0.5", className)}
      onKeyDown={event => {
        onKeyDown?.(event);
        if (event.defaultPrevented) return;
        selection.onKeyDown(event);
        if (event.key === "Enter" || event.key === " ") event.stopPropagation();
      }}
    >
      <hr className="mx-1.5 my-1 h-px basis-full border-0 bg-line-soft" />
      {/* One flat keyed sibling list: the "mode" control stays the same DOM
          node across both modes, so keyboard focus survives entering and
          leaving selection. */}
      {selection.mode ? (
        <span
          key="count"
          aria-live="polite"
          data-testid="worktree-selection-count"
          className="min-w-0 flex-1 truncate pl-2 text-micro text-subtle tabular-nums"
        >
          {selection.count} selected
        </span>
      ) : null}
      {selection.mode
        ? render(
            {
              id: "all",
              label: "Select all",
              run: selection.selectAll,
              disabled: selection.allEligibleSelected,
            },
            COMPACT_ACTION_CLASS
          )
        : null}
      {render(
        selection.mode
          ? { id: "mode", label: "Cancel", run: selection.clear, disabled: false }
          : {
              id: "mode",
              label: "Select worktrees…",
              run: selection.start,
              disabled: selection.eligibleCount === 0,
              icon: <ListChecks aria-hidden="true" className="size-3.5 text-faint" />,
            },
        selection.mode
          ? COMPACT_ACTION_CLASS
          : cn(FOOTER_ROW_CLASS, "w-full basis-full justify-start px-2 text-muted-foreground")
      )}
      {selection.mode
        ? render(
            {
              id: "confirm",
              label: worktreeRemoveSelectedLabel(selection.count),
              run: selection.confirm,
              disabled: selection.count === 0,
              destructive: true,
              icon: <Trash2 aria-hidden="true" className="size-3.5" />,
            },
            cn(FOOTER_ROW_CLASS, "w-full basis-full justify-start px-2")
          )
        : null}
    </div>
  );
}
