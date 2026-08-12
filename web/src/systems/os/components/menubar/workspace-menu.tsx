import { Check, Ellipsis, Info, Plus } from "lucide-react";

import {
  Alert,
  AlertDescription,
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarRadioGroup,
  MenubarRadioItem,
  MenubarSeparator,
} from "@compozy/ui";

import {
  GLOBAL_SCOPE_COPY,
  groupWorkspaceTree,
  truncateWorktreeNest,
  WorktreeAggregate,
  WorktreeRow,
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
}

/** Indent + rail. Depth is fixed at 2, so no row here can nest further. */
const NEST_ROW_CLASS = "ml-[15px] border-l border-line-strong pl-3";

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
          // A non-git workspace gets no nest at all — absent, never disabled.
          const nest = node?.gitBacked ? truncateWorktreeNest(node.worktrees) : null;
          return (
            <div key={workspace.id}>
              <MenubarItem
                data-testid={`os-workspace-option-${workspace.id}`}
                onClick={() => onSelectWorkspace(workspace.id)}
              >
                <span className="grid size-4 place-items-center rounded-xs border border-line-strong bg-elevated font-mono text-micro font-semibold">
                  {monogram(workspace.name)}
                </span>
                {workspace.name}
                <WorktreeAggregate runningAgents={node?.runningAgents ?? 0} />
                {!globalScopeOn && workspace.id === activeWorkspaceId ? (
                  <Check className="ml-auto size-3 text-accent" />
                ) : null}
              </MenubarItem>

              {/* Radio semantics come from the primitive: exactly one worktree
                  can scope a workspace, and `role="menu"` keeps holding only
                  menu items. Global cannot bind a worktree, so nothing is
                  selected while Global is on. */}
              {nest ? (
                <MenubarRadioGroup
                  value={globalScopeOn ? "" : (selectedWorktreeId ?? "")}
                  aria-label={`Worktrees in ${workspace.name}`}
                >
                  {nest.visible.map(entry => (
                    <MenubarRadioItem
                      key={entry.key}
                      value={entry.worktree?.id ?? entry.key}
                      disabled={!entry.selectable}
                      className={NEST_ROW_CLASS}
                      data-testid={`os-worktree-option-${entry.key}`}
                      onClick={() => {
                        if (!entry.selectable) return;
                        onSelectWorktree?.(workspace.id, entry);
                      }}
                    >
                      <WorktreeRow
                        entry={entry}
                        density="nest"
                        className="w-full px-0 hover:bg-transparent"
                        trailing={
                          entry.adoptable ? (
                            <span className="text-mono-id font-semibold text-info">Adopt</span>
                          ) : null
                        }
                      />
                    </MenubarRadioItem>
                  ))}
                  {nest.overflowCount > 0 ? (
                    <MenubarItem
                      className={NEST_ROW_CLASS}
                      data-testid={`os-worktree-overflow-${workspace.id}`}
                      onClick={onOpenWorkspaces}
                    >
                      <Ellipsis className="size-3 text-faint" />
                      All {node?.adoptedCount ?? 0} worktrees
                    </MenubarItem>
                  ) : null}
                  {onCreateWorktree ? (
                    <MenubarItem
                      className={NEST_ROW_CLASS}
                      data-testid={`os-worktree-create-${workspace.id}`}
                      onClick={() => onCreateWorktree(workspace.id)}
                    >
                      <Plus className="size-3 text-faint" />
                      New worktree
                    </MenubarItem>
                  ) : null}
                </MenubarRadioGroup>
              ) : null}
            </div>
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
