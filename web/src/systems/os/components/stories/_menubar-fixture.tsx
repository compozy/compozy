import { useState } from "react";
import { fn } from "storybook/test";

import { AgentCreateHostProvider } from "@/systems/agent";
import { DESTINATION_COPY } from "@/systems/workspace";
import type {
  ActiveWorktreeSelection,
  BreadthMode,
  Focus,
  WorkspaceChipIdentity,
  WorkspacePayload,
  WorktreesResponse,
} from "@/systems/workspace";
import {
  discoveredWorktreeFixture,
  worktreeListingFixture,
  worktreeMissingFixture,
  worktreePendingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { OsShellContext } from "../../contexts/os-shell-context";
import { cmdPaletteStoryRegistry } from "../../mocks/cmd-palette-fixtures";
import type { OsAttentionModel } from "../../hooks/use-os-attention";
import type { DesktopOverlay } from "../../hooks/use-desktop-overlays";
import { DesktopMenubar } from "../desktop-menubar";
import type { MenubarUpdateModel } from "../menubar/menubar-update-indicator";
import { DesktopPager, type DesktopPagerItem } from "../desktop-pager";
import { createLiveStoryShell, createStoryShell } from "./_shell-fixture";
import { DesktopShell } from "./_desktop";

/** Shared menubar story fixtures: the wired DesktopMenubar over a live desktop. */
export const WORKSPACES: WorkspacePayload[] = [
  {
    id: "workspace-compozy",
    name: "compozy",
    root_dir: "/workspace/compozy",
    add_dirs: [],
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:00:00Z",
  },
  {
    id: "workspace-site",
    name: "site",
    root_dir: "/workspace/site",
    add_dirs: [],
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:00:00Z",
  },
];

const ATTENTION: OsAttentionModel = {
  badges: { sessions: 1, tasks: 1 },
  notificationCount: 2,
  sections: { needsYou: [], finished: [] },
  sessions: [],
  attentionSessionsDisconnected: false,
  sessionsDisconnected: false,
  tasksDisconnected: false,
  loopRequestsDisconnected: false,
  loading: false,
};

/** Four desktops; Review holds a session that needs you (orange dot while off-screen). */
const PAGER_DESKTOPS: DesktopPagerItem[] = [
  { id: "control", name: "Control" },
  { id: "build", name: "Build" },
  { id: "review", name: "Review", needsYou: true },
  { id: "research", name: "Research" },
];

export function MenubarFixture({
  overlay = null,
  live = true,
  breadthMode,
  focus,
  chip,
  workspaces = WORKSPACES,
  onToggleAllProjects,
  worktreeSelection,
  listing = MENUBAR_WORKTREE_LISTING,
  update = { state: "hidden" },
}: {
  overlay?: DesktopOverlay | null;
  live?: boolean;
  breadthMode?: BreadthMode;
  focus?: Focus;
  chip?: WorkspaceChipIdentity;
  workspaces?: WorkspacePayload[];
  onToggleAllProjects?: () => void;
  worktreeSelection?: ActiveWorktreeSelection;
  listing?: WorktreesResponse;
  update?: MenubarUpdateModel;
}) {
  const [shell] = useState(() => (live ? createLiveStoryShell() : createStoryShell()));
  const [active, setActive] = useState<DesktopOverlay | null>(overlay);
  return (
    <OsShellContext.Provider value={shell}>
      <CmdPaletteRegistryProvider registry={cmdPaletteStoryRegistry}>
        <AgentCreateHostProvider openDialog={fn()} openForDuplicate={fn()}>
          <DesktopShell
            wallpaper="carbon"
            deskHint
            topbar={
              <DesktopMenubar
                workspaces={workspaces}
                activeWorkspace={
                  breadthMode === undefined && focus === undefined ? workspaces[0] : undefined
                }
                chip={chip}
                breadthMode={breadthMode}
                focus={focus}
                focusWorkspaceName={WORKSPACES[0].name}
                onToggleAllProjects={onToggleAllProjects}
                onSelectWorkspace={fn()}
                onFocusUser={fn()}
                onEnableAllProjects={fn()}
                onAddWorkspace={fn()}
                onRunCommand={fn()}
                activeOverlay={active}
                onOverlayOpenChange={(id, open) =>
                  setActive(current => (open ? id : current === id ? null : current))
                }
                attention={ATTENTION}
                update={update}
                worktreesByWorkspace={{ [WORKSPACES[0].id]: listing }}
                userHomeDir="/Users/ada"
                worktreeSelection={worktreeSelection}
                onSelectWorktree={fn()}
                onCreateWorktree={fn()}
                pager={
                  <DesktopPager
                    desktops={PAGER_DESKTOPS}
                    activeDesktopId="build"
                    onSelectDesktop={fn()}
                    onOpenOverview={fn()}
                  />
                }
              />
            }
          />
        </AgentCreateHostProvider>
      </CmdPaletteRegistryProvider>
    </OsShellContext.Provider>
  );
}

function remapWorkspaceId<T extends { workspace_id?: string }>(entry: T): T {
  return { ...entry, workspace_id: WORKSPACES[0].id };
}

export const MENUBAR_WORKTREE_LISTING: WorktreesResponse = {
  ...worktreeListingFixture,
  worktrees: worktreeListingFixture.worktrees.map(remapWorkspaceId),
};

/** ≤5 rows so discovered and missing survive the nest truncate. */
export const MENUBAR_NEST_LISTING: WorktreesResponse = {
  ...worktreeListingFixture,
  worktrees: [worktreeReadyDirtyRunningFixture, worktreePendingFixture, worktreeMissingFixture].map(
    remapWorkspaceId
  ),
  discovered: [discoveredWorktreeFixture],
};

export const READY_WORKTREE = MENUBAR_WORKTREE_LISTING.worktrees.find(
  worktree => worktree.state === "ready"
)!;
export const MISSING_WORKTREE = MENUBAR_WORKTREE_LISTING.worktrees.find(
  worktree => worktree.state === "missing"
)!;

export const ALL_PROJECTS_CHIP: WorkspaceChipIdentity = {
  kind: "all",
  name: DESTINATION_COPY.allProjects,
  monogram: DESTINATION_COPY.allProjectsMonogram,
};

export const USER_CHIP: WorkspaceChipIdentity = {
  kind: "user",
  name: DESTINATION_COPY.user,
  monogram: DESTINATION_COPY.userMonogram,
};
