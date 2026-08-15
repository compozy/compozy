import { useEffect, useRef } from "react";
import { Plus } from "lucide-react";

import { Button, Dialog, DialogContent, DialogDescription, DialogTitle, cn } from "@compozy/ui";

import {
  useWorkspacesSwitcher,
  WORKSPACES_ADD_TILE_KEY,
  type WorkspacesMenuNavRow,
  type WorkspacesSwitcherEntry,
} from "../hooks/use-workspaces-switcher";
import { useWorkspacesStripScroll } from "../hooks/use-workspaces-strip-scroll";
import { OsWorkspaceAddTile, OsWorkspaceTile } from "./os-workspace-tile";
import { OsWorkspacesCaption, type OsWorkspacesCaptionModel } from "./os-workspaces-caption";
import { OsWorkspacesHints } from "./os-workspaces-hints";
import { OsWorkspacesStrip } from "./os-workspaces-strip";
import { buildWorkspacesWorktreeMenuModel } from "../lib/workspaces-overview-model";
import { OsWorkspacesWorktreeMenu } from "./os-workspaces-worktree-menu";
import {
  contractHomePath,
  groupWorkspaceTree,
  workspaceMonogram,
  type WorkspacePayload,
  type WorkspaceScopeMode,
  type WorkspaceTreeNode,
  type WorktreeListingByWorkspace,
  type WorktreeNestEntry,
} from "@/systems/workspace";

export interface OsWorkspacesOverviewProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaces: WorkspacePayload[];
  activeWorkspaceId: string | null;
  /** While Global is on no tile is current and the notice line renders. */
  scope: WorkspaceScopeMode;
  onSelectWorkspace: (workspaceId: string) => void;
  onNewWorkspace: () => void;
  reducedMotion: boolean;
  /** Same projection as the switcher and the menubar menu. */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
  userHomeDir?: string;
  selectedWorktreeId?: string | null;
  onSelectWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
  onCreateWorktree?: (workspaceId: string) => void;
  onRemoveWorktree?: (workspaceId: string, entry: WorktreeNestEntry) => void;
}

/**
 * Workspace identity switching as a macOS Command-Tab overlay: a centered
 * glass strip of compact tiles over the live (inert) shell, with the focused
 * workspace's worktrees as a subordinate vertical menu. Window arrangements
 * stay with the dedicated Desktops Overview.
 */
export function OsWorkspacesOverview({ open, onOpenChange, ...stage }: OsWorkspacesOverviewProps) {
  const contentRef = useRef<HTMLDivElement | null>(null);
  // Escape steps one layer at a time: popover → menu → overlay. The stage
  // registers its layer guard here so the dialog's own escape close can be
  // vetoed while the worktree menu owns focus.
  const escapeGuardRef = useRef<() => boolean>(() => false);

  return (
    <Dialog
      open={open}
      onOpenChange={(next, details) => {
        if (!next && details?.reason === "escape-key" && escapeGuardRef.current()) return;
        onOpenChange(next);
      }}
    >
      <DialogContent
        ref={contentRef}
        unframed
        showCloseButton={false}
        data-testid="os-workspaces-overview"
        className={cn(
          "top-0 left-0 h-full w-full max-w-none translate-x-0 translate-y-0 overflow-hidden",
          // The dialog base caps content at sm:max-w-sm; beat it in its own
          // variant bucket — the overlay owns the whole viewport.
          "sm:max-w-none",
          "flex flex-col items-center justify-center rounded-none px-4 pt-6 pb-20",
          "bg-transparent shadow-none backdrop-blur-shell-scrim"
        )}
        onClick={event => {
          // Backdrop click closes; the popup is full-bleed, so "backdrop" is
          // the content root and the reserved band below the stage.
          const target = event.target as HTMLElement;
          if (target === event.currentTarget || target.dataset.osWsovDismiss !== undefined) {
            onOpenChange(false);
          }
        }}
      >
        <OsWorkspacesStage
          {...stage}
          overlayRef={contentRef}
          escapeGuardRef={escapeGuardRef}
          onOpenChange={onOpenChange}
        />
      </DialogContent>
    </Dialog>
  );
}

interface OsWorkspacesStageProps extends Omit<OsWorkspacesOverviewProps, "open"> {
  overlayRef: React.RefObject<HTMLDivElement | null>;
  escapeGuardRef: React.RefObject<() => boolean>;
}

const ADD_ENTRY: WorkspacesSwitcherEntry = {
  key: WORKSPACES_ADD_TILE_KEY,
  kind: "add",
  node: null,
};

function flatNode(workspace: WorkspacePayload): WorkspaceTreeNode<WorkspacePayload> {
  return { workspace, gitBacked: false, worktrees: [], adoptedCount: 0, runningAgents: 0 };
}

function subtitleFor(workspaceCount: number, totalWorktrees: number): string {
  return [
    `${workspaceCount} workspace${workspaceCount === 1 ? "" : "s"}`,
    // Zero renders nothing rather than "0 worktrees".
    totalWorktrees > 0 ? `${totalWorktrees} worktree${totalWorktrees === 1 ? "" : "s"}` : null,
  ]
    .filter(Boolean)
    .join(" · ");
}

function captionFor(
  entry: WorkspacesSwitcherEntry | undefined,
  userHomeDir: string | undefined
): OsWorkspacesCaptionModel {
  if (!entry || entry.kind === "add") {
    return {
      key: WORKSPACES_ADD_TILE_KEY,
      title: "New workspace",
      meta: "Register a project folder",
      path: "directory picker",
    };
  }
  const rootDir = entry.node?.workspace.root_dir;
  return {
    key: entry.key,
    title: entry.node?.workspace.name ?? "",
    path: rootDir ? contractHomePath(rootDir, userHomeDir) : rootDir,
  };
}

const GLOBAL_CAPTION: OsWorkspacesCaptionModel = {
  key: "global",
  title: "Global",
  meta: "visible to every workspace",
  path: "~",
};

function OsWorkspacesStage({
  workspaces,
  activeWorkspaceId,
  scope,
  onSelectWorkspace,
  onNewWorkspace,
  reducedMotion,
  worktreesByWorkspace,
  userHomeDir,
  selectedWorktreeId,
  onSelectWorktree,
  onCreateWorktree,
  onRemoveWorktree,
  onOpenChange,
  overlayRef,
  escapeGuardRef,
}: OsWorkspacesStageProps) {
  const tree = worktreesByWorkspace
    ? groupWorkspaceTree(workspaces, worktreesByWorkspace, userHomeDir)
    : workspaces.map(flatNode);
  const empty = tree.length === 0;
  const entries: WorkspacesSwitcherEntry[] = empty
    ? []
    : [
        ...tree.map(node => ({ key: node.workspace.id, kind: "workspace" as const, node })),
        ADD_ENTRY,
      ];
  const totalWorktrees = tree.reduce((total, node) => total + node.adoptedCount, 0);
  const canCreate = Boolean(onCreateWorktree);
  const menuModelByKey = new Map(
    tree.map(node => [node.workspace.id, buildWorkspacesWorktreeMenuModel(node, canCreate)])
  );

  const activateEntry = (entry: WorkspacesSwitcherEntry) => {
    if (entry.kind === "add") {
      onOpenChange(false);
      onNewWorkspace();
      return;
    }
    // Enter on a tile always lands on the workspace root; while Global is on
    // the store turns Global off in the same gesture (workspaceSelected).
    onSelectWorkspace(entry.key);
    onOpenChange(false);
  };

  const activateMenuRow = (entry: WorkspacesSwitcherEntry, row: WorkspacesMenuNavRow) => {
    if (row.kind === "create") {
      onOpenChange(false);
      onCreateWorktree?.(entry.key);
      return;
    }
    if (!row.entry) return;
    // A discovered row is the adoption gesture; anything else scopes — both
    // decisions live with the shell handler.
    onSelectWorktree?.(entry.key, row.entry);
    onOpenChange(false);
  };

  // Settle-snapping and the escape guard resolve through refs: the scroll
  // hook and the dialog need them before/outside the switcher's creation.
  const settleRef = useRef<(index: number) => void>(() => {});
  const scroll = useWorkspacesStripScroll({
    overlayRef,
    reducedMotion,
    itemCount: entries.length,
    onSettleFocus: index => settleRef.current(index),
  });

  const switcher = useWorkspacesSwitcher({
    entries,
    menuNavRowsForKey: key => menuModelByKey.get(key)?.navRows ?? [],
    initialFocusKey: scope === "workspace" ? activeWorkspaceId : null,
    scopedRowKeyForKey: key =>
      scope === "workspace" && key === activeWorkspaceId ? (selectedWorktreeId ?? null) : null,
    reducedMotion,
    onActivateEntry: activateEntry,
    onActivateMenuRow: activateMenuRow,
    keepInView: scroll.keepInView,
    consumeDragClick: scroll.consumeDragClick,
    isTrackDragging: scroll.isTrackDragging,
  });

  useEffect(() => {
    settleRef.current = switcher.focusNearest;
    escapeGuardRef.current = switcher.guardEscape;
  });

  const focusedEntry = switcher.focusedEntry;
  const focusedMenuKey = focusedEntry?.kind === "workspace" ? focusedEntry.key : null;
  const menuModel = focusedMenuKey ? (menuModelByKey.get(focusedMenuKey) ?? null) : null;
  const menuNavRows = menuModel?.navRows ?? [];
  const scopedRowKey =
    scope === "workspace" && focusedMenuKey === activeWorkspaceId
      ? (selectedWorktreeId ?? null)
      : null;
  const focusedRowKey =
    switcher.layer === "menu" ? (menuNavRows[switcher.menuIndex]?.key ?? null) : null;

  return (
    <>
      <div
        data-slot="os-workspaces-stage"
        className={cn(
          "flex w-[min(1080px,calc(100%-32px))] flex-col items-center",
          !reducedMotion && "os-wsov-in"
        )}
        onKeyDown={switcher.onStageKeyDown}
      >
        <DialogTitle className="sr-only">Workspaces</DialogTitle>
        <DialogDescription
          className="mb-3.5 text-center text-form-label text-muted"
          data-testid="os-workspaces-subtitle"
        >
          {subtitleFor(tree.length, totalWorktrees)}
        </DialogDescription>
        {!empty && scope === "global" ? (
          <p
            data-testid="os-workspaces-notice"
            className="-mt-1.5 mb-3.5 max-w-[42ch] text-center text-form-label text-muted"
          >
            Global scope is on — picking a workspace turns it off.
          </p>
        ) : null}
        {empty ? (
          <div
            data-slot="os-workspaces-strip"
            data-testid="os-workspaces-strip"
            className={cn(
              "flex flex-col items-center gap-2.5 rounded-xl border border-line-strong px-6 py-5",
              "bg-shell-glass-pop text-center shadow-shell-strip backdrop-blur-shell-strip backdrop-saturate-[1.25]"
            )}
          >
            <p className="max-w-[28ch] text-small-body text-muted">
              No project folders yet. Sessions run in Global until you add one.
            </p>
            {/* The one primary on this surface — there is nothing to switch to. */}
            <Button
              data-testid="os-workspaces-new"
              size="sm"
              type="button"
              onClick={() => activateEntry(ADD_ENTRY)}
            >
              <Plus aria-hidden="true" className="size-3" />
              New workspace
            </Button>
          </div>
        ) : (
          <OsWorkspacesStrip
            overflow={scroll.overflow}
            dragging={scroll.dragging}
            reducedMotion={reducedMotion}
            trackRef={scroll.trackRef}
            trackProps={scroll.trackProps}
          >
            {entries.map((entry, index) => {
              const focused = switcher.layer !== "menu" && index === switcher.focusIndex;
              const anchorsMenu = index === switcher.focusIndex;
              if (entry.kind === "add") {
                return (
                  <OsWorkspaceAddTile
                    key={entry.key}
                    data-testid="os-workspace-tile-add"
                    focused={anchorsMenu}
                    tabIndex={focused ? 0 : -1}
                    ref={switcher.registerTile(entry.key)}
                    {...switcher.tileHandlers(index)}
                  />
                );
              }
              const name = entry.node?.workspace.name ?? "";
              return (
                <OsWorkspaceTile
                  key={entry.key}
                  data-testid={`os-workspace-tile-${entry.key}`}
                  name={name}
                  monogram={workspaceMonogram(name)}
                  // The plate stays on the focused tile while the menu owns
                  // key focus — it anchors which workspace the menu belongs to.
                  focused={anchorsMenu}
                  tabIndex={focused ? 0 : -1}
                  current={
                    scope === "workspace" && entry.key === activeWorkspaceId
                      ? selectedWorktreeId
                        ? "wt"
                        : "root"
                      : null
                  }
                  ref={switcher.registerTile(entry.key)}
                  {...switcher.tileHandlers(index)}
                />
              );
            })}
          </OsWorkspacesStrip>
        )}
        <OsWorkspacesCaption
          model={empty ? GLOBAL_CAPTION : captionFor(focusedEntry, userHomeDir)}
          reducedMotion={reducedMotion}
        />
        {/* Zero-height slot: the menu hangs under the caption without ever
            shifting the centered strip as its height changes. */}
        <div
          data-slot="os-workspaces-below"
          className="flex h-0 w-full items-start justify-center overflow-visible"
        >
          {menuModel ? (
            <OsWorkspacesWorktreeMenu
              model={menuModel}
              userHomeDir={userHomeDir}
              focusedRowKey={focusedRowKey}
              scopedRowKey={scopedRowKey}
              reducedMotion={reducedMotion}
              registerRow={switcher.registerRow}
              rowHandlers={switcher.menuRowHandlers}
              canCreate={canCreate}
              onDeleteRow={entry => {
                if (focusedMenuKey) onRemoveWorktree?.(focusedMenuKey, entry);
              }}
            />
          ) : null}
        </div>
      </div>
      {/* Reserved band below the stage: keeps the strip above dead center so
          the tallest worktree menu never shifts it. Part of the backdrop. */}
      <span
        aria-hidden="true"
        data-os-wsov-dismiss=""
        className="w-full flex-none basis-[clamp(160px,30vh,300px)]"
      />
      <OsWorkspacesHints
        layer={switcher.layer === "menu" ? "menu" : "strip"}
        hasMenuRows={menuNavRows.length > 0}
        empty={empty}
      />
    </>
  );
}
