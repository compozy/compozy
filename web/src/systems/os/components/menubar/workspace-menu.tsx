import { Check, ChevronRight, Info } from "lucide-react";

import {
  Alert,
  AlertDescription,
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
} from "@compozy/ui";

import {
  GLOBAL_SCOPE_COPY,
  groupWorkspaceTree,
  WorktreeAggregate,
  WorktreeSubmenuPanel,
  WORKTREE_SUBMENU_FRAME_CLASS,
  type WorkspacePayload,
  type WorktreeListingByWorkspace,
  type WorktreeNestEntry,
} from "@/systems/workspace";

export interface WorkspaceMenuProps {
  /** The workspace chip, already built as a `MenubarTrigger` by the chrome. */
  trigger: React.ReactNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaces: WorkspacePayload[];
  activeWorkspaceId: string | undefined;
  globalScopeOn?: boolean;
  deletionNotice?: string | null;
  monogram: (name: string) => string;
  onSelectWorkspace: (workspaceId: string) => void;
  onOpenWorkspaces: () => void;
  onAddWorkspace: () => void;
  /** Same query as the S1 switcher — the two surfaces must not diverge. */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
  userHomeDir?: string;
  selectedWorktreeId?: string | null;
  onSelectWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onCreateWorktree?: (workspaceId: string) => void;
  onResolveMissingWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onOpenWorktreeContext?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  /** Opens the remove dialog for an adopted worktree (row actions menu). */
  onRemoveWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
}

function WorkspaceRowLabel({
  monogram,
  name,
  runningAgents,
}: {
  monogram: string;
  name: string;
  runningAgents: number;
}) {
  return (
    <>
      <span className="grid size-4 shrink-0 place-items-center rounded-xs border border-line-strong bg-elevated font-mono text-micro font-semibold">
        {monogram}
      </span>
      {name}
      <WorktreeAggregate runningAgents={runningAgents} />
    </>
  );
}

/** Workspace switcher: the bound set, then the overview and creation paths. */
export function WorkspaceMenu({
  trigger,
  open,
  onOpenChange,
  workspaces,
  activeWorkspaceId,
  globalScopeOn = false,
  deletionNotice = null,
  monogram,
  onSelectWorkspace,
  onOpenWorkspaces,
  onAddWorkspace,
  worktreesByWorkspace,
  userHomeDir,
  selectedWorktreeId,
  onSelectWorktree,
  onCreateWorktree,
  onResolveMissingWorktree,
  onOpenWorktreeContext,
  onRemoveWorktree,
}: WorkspaceMenuProps) {
  const notice = deletionNotice ?? (globalScopeOn ? GLOBAL_SCOPE_COPY.menuNotice : null);
  const tree = worktreesByWorkspace
    ? groupWorkspaceTree(workspaces, worktreesByWorkspace, userHomeDir)
    : null;
  const nodeByWorkspaceId = new Map(tree?.map(node => [node.workspace.id, node]) ?? []);
  const orderedWorkspaces = tree?.map(node => node.workspace) ?? workspaces;

  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      {trigger}
      <MenubarContent align="start" data-testid="os-workspace-menu">
        {notice ? (
          <Alert
            className="mx-1.5 mb-1 px-2.5 py-2"
            data-testid="os-workspace-global-notice"
            role="note"
            variant="info"
          >
            <Info aria-hidden="true" className="size-3.5" />
            <AlertDescription className="text-form-hint">{notice}</AlertDescription>
          </Alert>
        ) : null}
        {orderedWorkspaces.map(workspace => {
          const node = nodeByWorkspaceId.get(workspace.id);
          // A non-git workspace gets no worktree affordance — absent, never disabled.
          const gitBacked = Boolean(node?.gitBacked);
          const isActive = !globalScopeOn && workspace.id === activeWorkspaceId;
          const rowLabel = (
            <WorkspaceRowLabel
              monogram={monogram(workspace.name)}
              name={workspace.name}
              runningAgents={node?.runningAgents ?? 0}
            />
          );
          if (!gitBacked || !node) {
            return (
              <MenubarItem
                key={workspace.id}
                data-testid={`os-workspace-option-${workspace.id}`}
                onClick={() => onSelectWorkspace(workspace.id)}
              >
                {rowLabel}
                {isActive ? <Check className="ml-auto size-3 text-accent" /> : null}
              </MenubarItem>
            );
          }
          return (
            <MenubarSub key={workspace.id}>
              <MenubarSubTrigger
                // The primitive appends its own ml-auto chevron; hide it so the
                // active check and the chevron share one trailing lane.
                className="[&>svg:last-child]:hidden"
                data-testid={`os-workspace-option-${workspace.id}`}
                onClick={event => {
                  // Enter and screen-reader activation (detail 0) keep opening
                  // the submenu per the menu contract; a pointer click selects.
                  if (event.detail === 0) return;
                  onSelectWorkspace(workspace.id);
                  onOpenChange(false);
                }}
              >
                {rowLabel}
                <span className="ml-auto flex shrink-0 items-center gap-1">
                  {isActive ? <Check className="size-3 text-accent" /> : null}
                  <ChevronRight aria-hidden="true" className="size-3 text-faint" />
                </span>
              </MenubarSubTrigger>
              <MenubarSubContent
                className={WORKTREE_SUBMENU_FRAME_CLASS}
                data-testid={`os-worktree-submenu-${workspace.id}`}
              >
                <WorktreeSubmenuPanel
                  node={node}
                  selectedWorktreeId={globalScopeOn ? null : selectedWorktreeId}
                  testIdPrefix="os"
                  variant="menu"
                  onSelectWorktree={
                    onSelectWorktree ? entry => onSelectWorktree(workspace.id, entry) : undefined
                  }
                  onCreateWorktree={
                    onCreateWorktree ? () => onCreateWorktree(workspace.id) : undefined
                  }
                  onResolveMissing={
                    onResolveMissingWorktree
                      ? entry => onResolveMissingWorktree(workspace.id, entry)
                      : undefined
                  }
                  onOpenContext={
                    onOpenWorktreeContext
                      ? entry => onOpenWorktreeContext(workspace.id, entry)
                      : undefined
                  }
                  onShowAllWorktrees={onOpenWorkspaces}
                  onRemoveWorktree={
                    onRemoveWorktree ? entry => onRemoveWorktree(workspace.id, entry) : undefined
                  }
                />
              </MenubarSubContent>
            </MenubarSub>
          );
        })}
        <MenubarSeparator />
        <MenubarItem data-testid="os-workspace-overview" onClick={onOpenWorkspaces}>
          Workspaces overview…
        </MenubarItem>
        <MenubarItem data-testid="os-workspace-add" onClick={onAddWorkspace}>
          Add workspace…
        </MenubarItem>
      </MenubarContent>
    </MenubarMenu>
  );
}
