import type { WorktreeRemovalBatch } from "@/systems/workspace";
import { Archive, Check, ChevronRight } from "@compozy/ui/icons";

import {
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
} from "@compozy/ui";

import {
  DESTINATION_COPY,
  groupWorkspaceTree,
  WorkspaceWorkingDot,
  WorktreeAggregate,
  WorktreeSubmenuPanel,
  worktreeNestPresence,
  WORKTREE_SUBMENU_FRAME_CLASS,
  type BreadthMode,
  type Focus,
  type WorkspacePayload,
  type WorktreeListingByWorkspace,
  type WorktreeNestEntry,
  type WorkspaceProjectMenuPoint,
} from "@/systems/workspace";

import { openContextMenuFromKeyboard } from "../../lib/context-menu-keyboard";

import { MenubarCommandGroups } from "./menubar-command-groups";
import { MenubarCommandItem } from "./menubar-command-item";

export interface WorkspaceMenuProps {
  /** The workspace chip, already built as a `MenubarTrigger` by the chrome. */
  trigger: React.ReactNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaces: WorkspacePayload[];
  /** All projects is checked while on; otherwise the focus is. */
  breadthMode: BreadthMode;
  focus: Focus;
  onSelectWorkspace: (workspaceId: string) => void;
  onFocusUser?: () => void;
  onEnableAllProjects?: () => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
  onAddWorkspace: () => void;
  /** Same query as the S1 switcher — the two surfaces must not diverge. */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
  /** Working-session counts by workspace id; drives the dot after each name. */
  workingSessionsByWorkspace?: Readonly<Record<string, number>>;
  userHomeDir?: string;
  selectedWorktreeId?: string | null;
  onSelectWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onCreateWorktree?: (workspaceId: string) => void;
  onResolveMissingWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onOpenWorktreeContext?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  /** Opens the remove dialog for an adopted worktree (row actions menu). */
  onRemoveWorktrees?: (batch: WorktreeRemovalBatch) => void;
  onRemoveWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  /** Scope fallback sentence, shown first so the chip's warning dot has its reason. */
  notice?: string | null;
  /**
   * A context click on a project row. A menubar menu cannot nest a context
   * menu, so the host closes this menu and opens the project's at the point.
   */
  onProjectContextMenu?: (workspace: WorkspacePayload, point: WorkspaceProjectMenuPoint) => void;
  /**
   * Archive project from a project's submenu. The submenu closes with the
   * menu, so the host owns the confirm. Remove project stays off this hover menu.
   */
  onArchiveProject?: (workspace: WorkspacePayload) => void;
}

/** Right-click and Shift+F10 on a project row relay its project menu to the host. */
function projectRowMenuHandlers(
  workspace: WorkspacePayload,
  onProjectContextMenu: WorkspaceMenuProps["onProjectContextMenu"]
) {
  if (!onProjectContextMenu) return {};
  return {
    onContextMenu: (event: React.MouseEvent<HTMLElement>) => {
      event.preventDefault();
      onProjectContextMenu(workspace, { x: event.clientX, y: event.clientY });
    },
    onKeyDown: openContextMenuFromKeyboard,
  };
}

/** The name alone, like the topbar chip, then what is working there. */
function WorkspaceRowLabel({
  name,
  runningAgents,
  workingSessions,
}: {
  name: string;
  runningAgents: number;
  workingSessions: number;
}) {
  return (
    <>
      <span className="min-w-0 truncate">{name}</span>
      <WorkspaceWorkingDot count={workingSessions} placement="inline" />
      <WorktreeAggregate runningAgents={runningAgents} />
    </>
  );
}

function ActiveCheck({ active }: { active: boolean }) {
  return active ? <Check className="ml-auto size-3.5 text-brand" /> : null;
}

/** The All projects and User rows: destinations, never projects, with no worktree submenu. */
function DestinationRows({
  breadthMode,
  focus,
  onFocusUser,
  onEnableAllProjects,
}: Pick<WorkspaceMenuProps, "breadthMode" | "focus" | "onFocusUser" | "onEnableAllProjects">) {
  return (
    <>
      <MenubarItem
        data-testid="os-destination-all-projects"
        onClick={() => onEnableAllProjects?.()}
      >
        {DESTINATION_COPY.allProjects}
        <ActiveCheck active={breadthMode === "all"} />
      </MenubarItem>
      <MenubarItem data-testid="os-destination-user" onClick={() => onFocusUser?.()}>
        {DESTINATION_COPY.user}
        <ActiveCheck active={breadthMode === "focused" && focus.kind === "user"} />
      </MenubarItem>
    </>
  );
}

/**
 * Destination switcher in order: All projects, User, then the projects (each
 * with its worktrees), then the overview and creation paths.
 */
export function WorkspaceMenu({
  trigger,
  open,
  onOpenChange,
  workspaces,
  breadthMode,
  focus,
  onSelectWorkspace,
  onFocusUser,
  onEnableAllProjects,
  onRun,
  onAddWorkspace,
  worktreesByWorkspace,
  workingSessionsByWorkspace,
  userHomeDir,
  selectedWorktreeId,
  onSelectWorktree,
  onCreateWorktree,
  onResolveMissingWorktree,
  onOpenWorktreeContext,
  onRemoveWorktree,
  onRemoveWorktrees,
  notice,
  onProjectContextMenu,
  onArchiveProject,
}: WorkspaceMenuProps) {
  const focusedProjectId =
    breadthMode === "focused" && focus.kind === "project" ? focus.workspaceId : undefined;
  const tree = worktreesByWorkspace ? groupWorkspaceTree(workspaces, worktreesByWorkspace) : null;
  const nodeByWorkspaceId = new Map(tree?.map(node => [node.workspace.id, node]) ?? []);
  const orderedWorkspaces = tree?.map(node => node.workspace) ?? workspaces;

  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      {trigger}
      <MenubarContent align="start" data-testid="os-workspace-menu">
        <MenubarCommandGroups
          groups={[
            ...(notice
              ? [
                  {
                    id: "notice",
                    content: (
                      <MenubarItem disabled data-testid="os-workspace-menu-notice">
                        {notice}
                      </MenubarItem>
                    ),
                  },
                ]
              : []),
            {
              id: "destinations",
              content: (
                <DestinationRows
                  breadthMode={breadthMode}
                  focus={focus}
                  onFocusUser={onFocusUser}
                  onEnableAllProjects={onEnableAllProjects}
                />
              ),
            },
            {
              id: "workspaces",
              content:
                orderedWorkspaces.length > 0 ? (
                  <>
                    {orderedWorkspaces.map(workspace => {
                      const node = nodeByWorkspaceId.get(workspace.id);
                      // Shared absence rule: a non-git workspace gets no worktree
                      // affordance — absent, never disabled.
                      const presence = node
                        ? worktreeNestPresence(node, Boolean(onCreateWorktree))
                        : "absent";
                      const isActive = workspace.id === focusedProjectId;
                      const rowLabel = (
                        <WorkspaceRowLabel
                          name={workspace.name}
                          runningAgents={node?.runningAgents ?? 0}
                          workingSessions={workingSessionsByWorkspace?.[workspace.id] ?? 0}
                        />
                      );
                      if (!node || presence === "absent") {
                        return (
                          <MenubarItem
                            key={workspace.id}
                            data-testid={`os-workspace-option-${workspace.id}`}
                            onClick={() => onSelectWorkspace(workspace.id)}
                            {...projectRowMenuHandlers(workspace, onProjectContextMenu)}
                          >
                            {rowLabel}
                            <ActiveCheck active={isActive} />
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
                            {...projectRowMenuHandlers(workspace, onProjectContextMenu)}
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
                              {isActive ? <Check className="size-3.5 text-brand" /> : null}
                              <ChevronRight aria-hidden="true" className="size-3.5 text-faint" />
                            </span>
                          </MenubarSubTrigger>
                          <MenubarSubContent
                            className={WORKTREE_SUBMENU_FRAME_CLASS}
                            data-testid={`os-worktree-submenu-${workspace.id}`}
                          >
                            <WorktreeSubmenuPanel
                              node={node}
                              onRemoveWorktrees={onRemoveWorktrees}
                              selectedWorktreeId={isActive ? selectedWorktreeId : null}
                              testIdPrefix="os"
                              variant="menu"
                              userHomeDir={userHomeDir}
                              onSelectWorktree={
                                onSelectWorktree
                                  ? entry => onSelectWorktree(workspace.id, entry)
                                  : undefined
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
                              onRemoveWorktree={
                                onRemoveWorktree
                                  ? entry => onRemoveWorktree(workspace.id, entry)
                                  : undefined
                              }
                            />
                            {onArchiveProject && !workspace.archived_at ? (
                              <>
                                <MenubarSeparator />
                                <MenubarItem
                                  className="min-h-control-compact text-muted-foreground"
                                  data-testid={`os-workspace-archive-${workspace.id}`}
                                  onClick={() => onArchiveProject(workspace)}
                                >
                                  <Archive className="size-3.5 text-faint" />
                                  Archive project
                                </MenubarItem>
                              </>
                            ) : null}
                          </MenubarSubContent>
                        </MenubarSub>
                      );
                    })}
                  </>
                ) : null,
            },
            {
              id: "manage",
              content: (
                <>
                  <MenubarCommandItem commandId="workspace.picker" onRun={onRun} />
                  <MenubarItem data-testid="os-workspace-add" onClick={onAddWorkspace}>
                    Add project…
                  </MenubarItem>
                </>
              ),
            },
          ]}
        />
      </MenubarContent>
    </MenubarMenu>
  );
}
