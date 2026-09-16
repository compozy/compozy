import type { ComponentProps } from "react";
import { Button, MenubarItem, cn } from "@compozy/ui";
import type { WorktreeRemovalSelection } from "@/systems/workspace/hooks/use-worktree-removal-selection";

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
  const actions = selection.mode
    ? [
        {
          id: "confirm",
          label: `Remove selected (${selection.count})…`,
          run: selection.confirm,
          disabled: selection.count === 0,
        },
        {
          id: "all",
          label: `Select all eligible (${selection.eligibleCount})`,
          run: selection.selectAll,
          disabled: selection.eligibleCount === 0,
        },
        { id: "mode", label: "Clear selection", run: selection.clear, disabled: false },
      ]
    : [
        {
          id: "mode",
          label: "Select worktrees…",
          run: selection.start,
          disabled: selection.eligibleCount === 0,
        },
      ];
  return (
    <div
      {...props}
      className={cn("flex flex-wrap gap-1 p-1", className)}
      onKeyDown={event => {
        onKeyDown?.(event);
        if (event.defaultPrevented) return;
        selection.onKeyDown(event);
        if (event.key === "Enter" || event.key === " ") event.stopPropagation();
      }}
    >
      {actions.map(action =>
        menu ? (
          <MenubarItem
            key={action.id}
            closeOnClick={false}
            disabled={action.disabled}
            onClick={action.run}
          >
            {action.label}
          </MenubarItem>
        ) : (
          <Button
            key={action.id}
            size="sm"
            variant="ghost"
            disabled={action.disabled}
            onClick={action.run}
          >
            {action.label}
          </Button>
        )
      )}
      {selection.mode ? (
        <p className="px-2 text-form-hint text-subtle">
          Select all includes only eligible rows in this list. New entries are not added
          automatically.
        </p>
      ) : null}
    </div>
  );
}
