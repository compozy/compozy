import { ChevronRight, ChevronsUpDown, Plus } from "lucide-react";

import {
  cn,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  CommandSeparator,
} from "@compozy/ui";

import { useWorkspaceCommandSelectState } from "../hooks/use-workspace-command-select-state";
import type { WorktreeListingByWorkspace } from "../lib/workspace-tree";
import type { WorktreeNestEntry } from "../lib/worktree-display";
import { WorktreeAggregate } from "./worktree-aggregate";
import { WorktreeHoverSubmenu } from "./worktree-hover-submenu";
import { WorktreeSubmenuPanel } from "./worktree-submenu-panel";

function workspaceInitial(name: string): string {
  return name.charAt(0).toUpperCase() || "·";
}

export interface WorkspaceCommandSelectOption {
  id: string;
  name: string;
  root_dir?: string | null;
}

export interface WorkspaceCommandSelectProps {
  userHomeDir?: string;
  workspaces: ReadonlyArray<WorkspaceCommandSelectOption> | undefined;
  value: string | null;
  onChange: (id: string) => void;
  onAddWorkspace?: () => void;
  disabled?: boolean;
  className?: string;
  ariaLabelledBy?: string;
  triggerTestId?: string;
  testIdPrefix?: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  size?: "default" | "compact";
  /**
   * Worktree listings keyed by workspace id. Omitted entirely on surfaces that
   * do not read worktrees, which keeps the switcher exactly as it was.
   */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
  selectedWorktreeId?: string | null;
  /** Selecting a discovered entry is the adoption gesture (ADR-002). */
  onSelectWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onCreateWorktree?: (workspaceId: string) => void;
  onRemoveWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
}

export function WorkspaceCommandSelect({
  userHomeDir,
  workspaces,
  value,
  onChange,
  onAddWorkspace,
  disabled,
  className,
  ariaLabelledBy,
  triggerTestId = "workspace-switcher",
  testIdPrefix = "workspace-command",
  open: openProp,
  onOpenChange,
  size = "default",
  worktreesByWorkspace,
  selectedWorktreeId,
  onSelectWorktree,
  onCreateWorktree,
  onRemoveWorktree,
}: WorkspaceCommandSelectProps) {
  const state = useWorkspaceCommandSelectState({
    userHomeDir,
    workspaces,
    value,
    onChange,
    onAddWorkspace,
    open: openProp,
    onOpenChange,
    worktreesByWorkspace,
  });
  const label = state.selected?.name ?? "No workspace";
  const hasWorkspaces = state.projectWorkspaces.length > 0;
  const isDisabled = disabled || !hasWorkspaces;

  return (
    <CommandSelect open={state.open} onOpenChange={state.changeOpen}>
      <CommandSelectTrigger
        aria-haspopup="listbox"
        aria-expanded={state.open}
        aria-labelledby={ariaLabelledBy}
        aria-label={hasWorkspaces ? `Workspace: ${label}` : "No workspace"}
        data-size={size}
        data-testid={triggerTestId}
        disabled={isDisabled}
        selected={Boolean(state.selected)}
        className={cn(
          size === "compact"
            ? "h-[calc(var(--height-pill-group-segment-md)+2*var(--space-pill-group-track-padding))] min-w-0 gap-1.5 rounded-md border-transparent bg-transparent px-(--space-pill-group-segment-md-x) py-0 text-subtle shadow-none hover:bg-row-hover hover:text-fg-strong focus-visible:border-transparent focus-visible:shadow-focus-ring [&>svg:last-child]:hidden"
            : "h-12 w-full gap-2.5 border-0 bg-transparent px-2 py-0 shadow-none hover:bg-hover focus-visible:border-0 focus-visible:shadow-none [&>svg:last-child]:hidden",
          className
        )}
      >
        <span
          className={cn(
            "flex min-w-0 flex-1 items-center text-left",
            size === "compact" ? "gap-1.5" : "gap-2.5"
          )}
        >
          <span
            aria-hidden="true"
            data-testid="workspace-switcher-avatar"
            className={cn(
              "inline-flex shrink-0 items-center justify-center rounded-sm font-mono text-eyebrow font-medium tracking-mono text-fg",
              size === "compact" ? "size-4 bg-canvas-tint" : "size-button-icon-xs bg-elevated"
            )}
          >
            {workspaceInitial(label)}
          </span>
          <span
            data-testid="workspace-switcher-name"
            className={cn(
              "min-w-0 flex-1 truncate font-medium text-fg",
              size === "compact"
                ? "text-form-label tracking-eyebrow"
                : "text-small-body tracking-normal"
            )}
          >
            {label}
          </span>
          <ChevronsUpDown
            aria-hidden="true"
            data-testid="workspace-switcher-chevron"
            className="size-3 shrink-0 text-subtle"
          />
        </span>
      </CommandSelectTrigger>
      <CommandSelectShell
        className={cn(size === "compact" ? "min-w-56" : "min-w-64")}
        inputPlaceholder="Search workspaces..."
        commandProps={{
          value: state.activeCommandValue,
          onValueChange: state.changeCommandValue,
          onKeyDown: state.handleCommandKeyDown,
        }}
        inputProps={{ ref: state.inputRef, "data-testid": `${testIdPrefix}-input` }}
      >
        <CommandList>
          <CommandEmpty data-testid={`${testIdPrefix}-empty`}>
            No workspaces match your search.
          </CommandEmpty>
          <CommandSelectGroup heading="Workspaces" data-testid={`${testIdPrefix}-group`}>
            {state.projectWorkspaces.map(workspace => {
              const isActive = workspace.id === value;
              const node = state.nodeByWorkspaceId.get(workspace.id);
              const gitBacked = Boolean(node?.gitBacked);
              const row = (
                <CommandItem
                  key={workspace.id}
                  value={workspace.name}
                  aria-haspopup={gitBacked ? "dialog" : undefined}
                  aria-expanded={gitBacked ? state.submenuWorkspaceId === workspace.id : undefined}
                  onPointerDown={event => {
                    if (event.button !== 0) return;
                    state.pointerSelectionArmed();
                  }}
                  onSelect={() => {
                    state.selectWorkspace(workspace.id, gitBacked);
                  }}
                  data-checked={isActive ? "true" : "false"}
                  data-testid={`${testIdPrefix}-item-${workspace.id}`}
                >
                  <span
                    aria-hidden="true"
                    data-testid={`${testIdPrefix}-item-avatar-${workspace.id}`}
                    className="inline-flex size-button-icon-xs shrink-0 items-center justify-center rounded-sm bg-elevated font-mono text-eyebrow font-medium tracking-mono text-fg"
                  >
                    {workspaceInitial(workspace.name)}
                  </span>
                  <span className="flex min-w-0 flex-1 items-center gap-2">
                    <span className="truncate text-small-body text-fg">{workspace.name}</span>
                    <WorktreeAggregate runningAgents={node?.runningAgents ?? 0} />
                  </span>
                  {gitBacked ? (
                    <ChevronRight
                      aria-hidden="true"
                      className="ml-auto size-3 shrink-0 text-faint"
                    />
                  ) : null}
                </CommandItem>
              );
              if (!gitBacked || !node) return row;
              return (
                <WorktreeHoverSubmenu
                  key={workspace.id}
                  open={state.submenuWorkspaceId === workspace.id}
                  onOpenChange={next => state.changeSubmenuOpen(workspace.id, next)}
                  testId={`${testIdPrefix}-worktree-submenu-${workspace.id}`}
                  label={`Worktrees in ${workspace.name}`}
                  trigger={row}
                  focusOnOpen={state.focusedSubmenuWorkspaceId === workspace.id}
                  onReturnFocus={() => state.inputRef.current?.focus()}
                >
                  <WorktreeSubmenuPanel
                    node={node}
                    selectedWorktreeId={selectedWorktreeId}
                    testIdPrefix={testIdPrefix}
                    onSelectWorktree={
                      onSelectWorktree ? entry => onSelectWorktree(workspace.id, entry) : undefined
                    }
                    onCreateWorktree={
                      onCreateWorktree ? () => onCreateWorktree(workspace.id) : undefined
                    }
                    onRemoveWorktree={
                      onRemoveWorktree ? entry => onRemoveWorktree(workspace.id, entry) : undefined
                    }
                    onClose={state.close}
                  />
                </WorktreeHoverSubmenu>
              );
            })}
          </CommandSelectGroup>
          {onAddWorkspace ? (
            <>
              <CommandSeparator />
              <CommandSelectGroup>
                <CommandItem
                  value="add workspace"
                  onSelect={state.addWorkspace}
                  data-testid={`${testIdPrefix}-add`}
                >
                  <Plus aria-hidden="true" className="size-4 shrink-0 text-subtle" />
                  <span className="text-small-body text-fg">Add workspace</span>
                </CommandItem>
              </CommandSelectGroup>
            </>
          ) : null}
        </CommandList>
      </CommandSelectShell>
    </CommandSelect>
  );
}
