import { useState } from "react";
import { ArrowRight, ChevronRight, Plus } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  Eyebrow,
  OwnerAvatar,
  Pill,
  cn,
} from "@compozy/ui";

import type {
  OsWorkspaceDetailModel,
  OsWorkspaceDetailsResult,
} from "../hooks/use-workspace-details";
import type { OsPresentation } from "../lib/os-types";
import {
  groupWorkspaceTree,
  WorktreeAggregate,
  WorktreeHoverSubmenu,
  WorktreeSubmenuPanel,
  type WorkspacePayload,
  type WorktreeListingByWorkspace,
  type WorktreeNestEntry,
} from "@/systems/workspace";

export interface OsWorkspacesOverviewProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaces: WorkspacePayload[];
  activeWorkspaceId: string | null;
  onSelectWorkspace: (workspaceId: string) => void;
  onNewWorkspace: () => void;
  details: OsWorkspaceDetailsResult;
  presentation: OsPresentation;
  reducedMotion: boolean;
  /** Same projection as the switcher and the menubar menu. */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
  userHomeDir?: string;
  selectedWorktreeId?: string | null;
  onSelectWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onCreateWorktree?: (workspaceId: string) => void;
  onRemoveWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onResolveMissing?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onOpenWorktreeContext?: (workspaceId: string, entry: WorktreeNestEntry) => void;
}

const MEMBER_AVATAR_MAX = 5;

function workspaceMonogram(name: string): string {
  return name.trim().slice(0, 2).toUpperCase() || "WS";
}

function workspaceMetaLine(
  detail: OsWorkspaceDetailModel | undefined,
  adoptedWorktrees: number
): string {
  if (!detail || detail.status === "loading") return "Loading…";
  if (detail.status === "error") return "Details unavailable";
  return [
    `${detail.agentCount} agent${detail.agentCount === 1 ? "" : "s"}`,
    `${detail.sessionCount} session${detail.sessionCount === 1 ? "" : "s"}`,
    // Zero renders nothing rather than "0 worktrees".
    adoptedWorktrees > 0
      ? `${adoptedWorktrees} worktree${adoptedWorktrees === 1 ? "" : "s"}`
      : null,
  ]
    .filter(Boolean)
    .join(" · ");
}

function WorkspaceMembers({ detail }: { detail: OsWorkspaceDetailModel | undefined }) {
  if (!detail || detail.status !== "ready" || detail.agentNames.length === 0) return null;
  const visible = detail.agentNames.slice(0, MEMBER_AVATAR_MAX);
  const overflow = detail.agentNames.length - visible.length;

  return (
    <span className="flex flex-col gap-1.5" data-slot="os-workspace-members">
      <Eyebrow className="text-faint">Members</Eyebrow>
      <span className="flex items-center">
        <span className="flex -space-x-1.5">
          {visible.map(name => (
            <OwnerAvatar
              className="ring-2 ring-canvas-soft"
              key={name}
              name={name}
              ownerId={name}
              ownerKind="agent"
              size="sm"
            />
          ))}
        </span>
        {overflow > 0 ? (
          <span className="ml-1.5 font-mono text-mono-id text-subtle">+{overflow}</span>
        ) : null}
      </span>
    </span>
  );
}

/**
 * Workspace switcher. Desktop layout intentionally does not leak into this
 * surface; the dedicated Desktops Overview owns window arrangements.
 */
export function OsWorkspacesOverview({
  open,
  onOpenChange,
  workspaces,
  activeWorkspaceId,
  onSelectWorkspace,
  onNewWorkspace,
  details,
  presentation,
  reducedMotion,
  worktreesByWorkspace,
  userHomeDir,
  selectedWorktreeId,
  onSelectWorktree,
  onCreateWorktree,
  onRemoveWorktree,
  onResolveMissing,
  onOpenWorktreeContext,
}: OsWorkspacesOverviewProps) {
  const [submenuWorkspaceId, setSubmenuWorkspaceId] = useState<string | null>(null);
  const selectWorkspace = (workspaceId: string) => {
    onSelectWorkspace(workspaceId);
    onOpenChange(false);
  };

  const tree = worktreesByWorkspace
    ? groupWorkspaceTree(workspaces, worktreesByWorkspace, userHomeDir)
    : null;
  const nodeByWorkspaceId = new Map(tree?.map(node => [node.workspace.id, node]) ?? []);
  const orderedWorkspaces = tree?.map(node => node.workspace) ?? workspaces;
  // Adopted-only, per the locked count rule: a discovered checkout has no
  // record and never enters a worktree count.
  const totalWorktrees = tree?.reduce((total, node) => total + node.adoptedCount, 0) ?? 0;

  const workspaceCount = workspaces.length;
  const subtitle = [
    `${workspaceCount} workspace${workspaceCount === 1 ? "" : "s"}`,
    details.totalAgents !== null
      ? `${details.totalAgents} agent${details.totalAgents === 1 ? "" : "s"}`
      : null,
    totalWorktrees > 0 ? `${totalWorktrees} worktree${totalWorktrees === 1 ? "" : "s"}` : null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        unframed
        showCloseButton={false}
        data-testid="os-workspaces-overview"
        className={cn(
          "top-0 left-0 h-full w-full max-w-none translate-x-0 translate-y-0 overflow-hidden",
          "flex flex-col items-center justify-center gap-2 rounded-none bg-transparent",
          "shadow-none backdrop-blur-shell-scrim"
        )}
      >
        <div className="flex w-full max-w-[calc(3*var(--width-workspace-card)+2*var(--spacing-workspaces-row-gap))] items-start justify-between gap-4 px-6">
          <div className="flex min-w-0 flex-col gap-1">
            <DialogTitle className="text-compact-h1 font-semibold text-fg-strong">
              Workspaces
            </DialogTitle>
            <DialogDescription
              className="text-small-body text-muted"
              data-testid="os-workspaces-subtitle"
            >
              {subtitle}
            </DialogDescription>
          </div>
          <Button
            data-testid="os-workspaces-new"
            onClick={() => {
              onOpenChange(false);
              onNewWorkspace();
            }}
            size="sm"
            type="button"
          >
            <Plus aria-hidden="true" className="size-3" />
            New workspace
          </Button>
        </div>
        <div
          data-slot="os-workspaces-row"
          className={cn(
            "mt-workspaces-subtitle-gap flex max-w-full justify-center",
            presentation === "compact"
              ? "max-h-[64vh] flex-col items-center gap-3 overflow-y-auto px-4 py-1"
              : "max-h-[70vh] flex-wrap gap-workspaces-row-gap overflow-y-auto px-6 py-1"
          )}
        >
          {orderedWorkspaces.map(workspace => {
            const active = workspace.id === activeWorkspaceId;
            const detail = details.byWorkspaceId.get(workspace.id);
            const node = nodeByWorkspaceId.get(workspace.id);
            const meta = workspaceMetaLine(detail, node?.adoptedCount ?? 0);
            return (
              // The card is a button, so the worktree trigger is a sibling:
              // interactive worktree rows cannot live inside a button.
              <div
                key={workspace.id}
                data-slot="os-workspace-card-group"
                className="flex w-workspace-card shrink-0 flex-col gap-1"
              >
                <button
                  type="button"
                  data-slot="os-workspace-card"
                  data-workspace-id={workspace.id}
                  data-current={active ? "true" : undefined}
                  aria-label={`${active ? "Current workspace" : "Switch to"} ${workspace.name}. ${meta}`}
                  className={cn(
                    "group flex w-full shrink-0 flex-col gap-3 overflow-hidden rounded-xl border bg-canvas-soft p-4 text-left",
                    "transition-[transform,border-color] duration-shell-fast ease-spring",
                    "focus-visible:shadow-focus-ring focus-visible:outline-none",
                    !reducedMotion && "hover:-translate-y-1",
                    active ? "border-accent" : "border-line-strong hover:border-line-focus"
                  )}
                  onClick={() => selectWorkspace(workspace.id)}
                >
                  <span className="flex items-center gap-2.5">
                    <span
                      aria-hidden="true"
                      className="grid size-7 shrink-0 place-items-center rounded-sm border border-line-strong bg-elevated font-mono text-workspace-avatar font-semibold text-muted"
                    >
                      {workspaceMonogram(workspace.name)}
                    </span>
                    <span className="min-w-0 flex-1 truncate text-small-body font-semibold text-fg-strong">
                      {workspace.name}
                    </span>
                    {active ? (
                      <Pill data-slot="os-workspace-current" size="xs" tone="neutral">
                        Current
                      </Pill>
                    ) : null}
                  </span>
                  <span
                    className="text-form-label text-subtle"
                    data-testid={`os-workspace-meta-${workspace.id}`}
                  >
                    {meta}
                  </span>
                  <WorkspaceMembers detail={detail} />
                  <span className="flex items-center justify-between border-t border-line-soft pt-2.5">
                    <span className="truncate font-mono text-mono-id text-faint">
                      {workspace.root_dir}
                    </span>
                    <span
                      className={cn(
                        "inline-flex shrink-0 items-center gap-1 text-form-label font-medium text-muted",
                        "transition-colors duration-base group-hover:text-fg"
                      )}
                    >
                      Enter
                      <ArrowRight aria-hidden="true" className="size-3" />
                    </span>
                  </span>
                </button>

                {node?.gitBacked ? (
                  <WorktreeHoverSubmenu
                    open={submenuWorkspaceId === workspace.id}
                    onOpenChange={next => setSubmenuWorkspaceId(next ? workspace.id : null)}
                    testId={`os-worktree-submenu-${workspace.id}`}
                    label={`Worktrees in ${workspace.name}`}
                    trigger={
                      <button
                        type="button"
                        className="flex min-h-6 items-center gap-1.5 self-start rounded-md px-2 text-form-label text-subtle hover:bg-row-hover"
                        aria-haspopup="dialog"
                        aria-expanded={submenuWorkspaceId === workspace.id}
                        aria-label={`Worktrees in ${workspace.name}`}
                        data-testid={`os-workspace-worktrees-toggle-${workspace.id}`}
                      >
                        <ChevronRight aria-hidden="true" className="size-3" />
                        Worktrees
                        <WorktreeAggregate runningAgents={node.runningAgents} />
                      </button>
                    }
                  >
                    <WorktreeSubmenuPanel
                      node={node}
                      limit={node.worktrees.length}
                      selectedWorktreeId={selectedWorktreeId}
                      testIdPrefix="os"
                      onSelectWorktree={
                        onSelectWorktree
                          ? entry => {
                              onSelectWorktree(workspace.id, entry);
                              onOpenChange(false);
                            }
                          : undefined
                      }
                      onCreateWorktree={
                        onCreateWorktree
                          ? () => {
                              onCreateWorktree(workspace.id);
                              onOpenChange(false);
                            }
                          : undefined
                      }
                      onRemoveWorktree={
                        onRemoveWorktree
                          ? entry => onRemoveWorktree(workspace.id, entry)
                          : undefined
                      }
                      onResolveMissing={
                        onResolveMissing
                          ? entry => onResolveMissing(workspace.id, entry)
                          : undefined
                      }
                      onOpenContext={
                        onOpenWorktreeContext
                          ? entry => onOpenWorktreeContext(workspace.id, entry)
                          : undefined
                      }
                    />
                  </WorktreeHoverSubmenu>
                ) : null}
              </div>
            );
          })}
        </div>
      </DialogContent>
    </Dialog>
  );
}
