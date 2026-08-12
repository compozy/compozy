import { Fragment, useState } from "react";
import { ChevronsUpDown, Ellipsis, Plus } from "lucide-react";

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

import { partitionProjectWorkspaces } from "../lib/project-workspaces";
import {
  groupWorkspaceTree,
  type WorkspaceTreeNode,
  type WorktreeListingByWorkspace,
} from "../lib/workspace-tree";
import type { WorktreeNestEntry } from "../lib/worktree-display";
import { truncateWorktreeNest } from "../lib/worktree-sort";
import { WorktreeAggregate } from "./worktree-nest-list";
import { WorktreeRow } from "./worktree-row";

function workspaceInitial(name: string): string {
  return name.charAt(0).toUpperCase() || "·";
}

/** Indent + 1px rail, drawn per row so cmdk can hide any row independently. */
const NEST_RAIL_CLASS =
  "ml-[15px] border-l border-line-strong pl-3 rounded-l-none data-[inert=true]:cursor-default";

interface WorktreeNestItemsOptions {
  node: WorkspaceTreeNode<WorkspaceCommandSelectOption>;
  selectedWorktreeId: string | null | undefined;
  testIdPrefix: string;
  onSelectWorktree: ((workspaceId: string, entry: WorktreeNestEntry) => void) | undefined;
  onCreateWorktree: ((workspaceId: string) => void) | undefined;
  onShowAllWorktrees: ((workspaceId: string) => void) | undefined;
  closeMenu: () => void;
}

/**
 * The nest, already sorted and truncated by the shared lib so this surface
 * cannot drift from the menubar menu or the overview.
 */
function renderWorktreeNestItems({
  node,
  selectedWorktreeId,
  testIdPrefix,
  onSelectWorktree,
  onCreateWorktree,
  onShowAllWorktrees,
  closeMenu,
}: WorktreeNestItemsOptions) {
  const { visible, overflowCount } = truncateWorktreeNest(node.worktrees);
  const workspaceId = node.workspace.id;

  return (
    <>
      {visible.map(entry => (
        <CommandItem
          key={entry.key}
          // Parent name is part of the search value so filtering by workspace
          // keeps its worktrees visible with it.
          value={`${node.workspace.name} ${entry.name} ${entry.branch}`}
          disabled={!entry.selectable}
          aria-disabled={entry.selectable ? undefined : true}
          data-testid={`${testIdPrefix}-worktree-${entry.key}`}
          className={NEST_RAIL_CLASS}
          onSelect={() => {
            if (!entry.selectable) return;
            onSelectWorktree?.(workspaceId, entry);
            closeMenu();
          }}
        >
          <WorktreeRow
            entry={entry}
            density="nest"
            selected={entry.worktree?.id === selectedWorktreeId}
            className="w-full px-0 hover:bg-transparent"
            trailing={
              entry.adoptable ? (
                <span className="text-mono-id font-semibold text-info">Adopt</span>
              ) : null
            }
          />
        </CommandItem>
      ))}

      {overflowCount > 0 && onShowAllWorktrees ? (
        <CommandItem
          value={`${node.workspace.name} all worktrees`}
          data-testid={`${testIdPrefix}-worktree-overflow-${workspaceId}`}
          className={NEST_RAIL_CLASS}
          onSelect={() => {
            onShowAllWorktrees(workspaceId);
            closeMenu();
          }}
        >
          <Ellipsis aria-hidden="true" className="size-3 shrink-0 text-faint" />
          <span className="text-form-label text-subtle">All {node.adoptedCount} worktrees</span>
        </CommandItem>
      ) : null}

      {onCreateWorktree ? (
        <CommandItem
          value={`${node.workspace.name} new worktree`}
          data-testid={`${testIdPrefix}-worktree-create-${workspaceId}`}
          className={NEST_RAIL_CLASS}
          onSelect={() => {
            onCreateWorktree(workspaceId);
            closeMenu();
          }}
        >
          <Plus aria-hidden="true" className="size-3 shrink-0 text-faint" />
          <span className="text-form-label text-subtle">New worktree</span>
        </CommandItem>
      ) : null}
    </>
  );
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
  onShowAllWorktrees?: (workspaceId: string) => void;
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
  onShowAllWorktrees,
}: WorkspaceCommandSelectProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const open = openProp ?? uncontrolledOpen;
  const setOpen = onOpenChange ?? setUncontrolledOpen;
  const { projectWorkspaces } = partitionProjectWorkspaces(workspaces, userHomeDir);
  const selected = projectWorkspaces.find(workspace => workspace.id === value) ?? null;
  // One projection behind every listing surface, so the switcher, the menubar
  // menu, and the overview cannot disagree about the same workspace.
  const tree = worktreesByWorkspace
    ? groupWorkspaceTree(projectWorkspaces, worktreesByWorkspace, userHomeDir)
    : null;
  const nodeByWorkspaceId = new Map(tree?.map(node => [node.workspace.id, node]) ?? []);
  const label = selected?.name ?? "No workspace";
  const hasWorkspaces = projectWorkspaces.length > 0;
  const isDisabled = disabled || !hasWorkspaces;

  const handleSelect = (workspace: WorkspaceCommandSelectOption) => {
    onChange(workspace.id);
    setOpen(false);
  };

  const handleAddWorkspace = () => {
    onAddWorkspace?.();
    setOpen(false);
  };

  return (
    <CommandSelect open={open} onOpenChange={next => setOpen(next)}>
      <CommandSelectTrigger
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-labelledby={ariaLabelledBy}
        aria-label={hasWorkspaces ? `Workspace: ${label}` : "No workspace"}
        data-size={size}
        data-testid={triggerTestId}
        disabled={isDisabled}
        selected={Boolean(selected)}
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
        inputProps={{ "data-testid": `${testIdPrefix}-input` }}
      >
        <CommandList>
          <CommandEmpty data-testid={`${testIdPrefix}-empty`}>
            No workspaces match your search.
          </CommandEmpty>
          <CommandSelectGroup heading="Workspaces" data-testid={`${testIdPrefix}-group`}>
            {projectWorkspaces.map(workspace => {
              const isActive = workspace.id === value;
              const node = nodeByWorkspaceId.get(workspace.id);
              return (
                <Fragment key={workspace.id}>
                  <CommandItem
                    value={workspace.name}
                    onSelect={() => handleSelect(workspace)}
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
                  </CommandItem>
                  {/* Worktree rows are real CommandItems rather than a nested
                    container, so cmdk's own filtering and arrow-key order apply
                    and keyboard traversal matches the visual tree for free.
                    A non-git workspace renders none of this — absent, not
                    disabled. */}
                  {node?.gitBacked
                    ? renderWorktreeNestItems({
                        node,
                        selectedWorktreeId,
                        testIdPrefix,
                        onSelectWorktree,
                        onCreateWorktree,
                        onShowAllWorktrees,
                        closeMenu: () => setOpen(false),
                      })
                    : null}
                </Fragment>
              );
            })}
          </CommandSelectGroup>
          {onAddWorkspace ? (
            <>
              <CommandSeparator />
              <CommandSelectGroup>
                <CommandItem
                  value="add workspace"
                  onSelect={handleAddWorkspace}
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
