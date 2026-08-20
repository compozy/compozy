import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { AgentCreateHostProvider } from "@/systems/agent";
import type { SettingsUpdateStatus } from "@/systems/settings";
import { settingsUpdateIndicatorAvailable } from "@/systems/settings";
import {
  settingsUpdateApplyingFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateStatusFixture,
} from "@/systems/settings/mocks";
import type {
  ActiveWorktreeSelection,
  WorkspacePayload,
  WorkspaceScopeMode,
  WorktreesResponse,
} from "@/systems/workspace";
import {
  discoveredWorktreeFixture,
  worktreeListingFixture,
  worktreeMissingFixture,
  worktreePendingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import { OsShellContext } from "../../contexts/os-shell-context";
import type { OsAttentionModel } from "../../hooks/use-os-attention";
import type { DesktopOverlay } from "../../hooks/use-desktop-overlays";
import { shortcutLabel } from "../../lib/window-manager-shortcuts";
import { DesktopMenubar } from "../desktop-menubar";
import { OsMenuBar } from "../os-menubar";
import { OsHydrationStatus } from "../os-hydration-status";
import { createLiveStoryShell, createStoryShell } from "./_shell-fixture";
import { DesktopShell } from "./_desktop";

const meta: Meta<typeof OsMenuBar> = {
  title: "systems/os/components/OsMenuBar",
  component: OsMenuBar,
  args: { commandShortcutLabel: shortcutLabel("meta+KeyK") },
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          'The desktop menubar: the CompozyOS system menu, the Global scope globe, the workspace switcher, the static Session / Go / Window / Help set, the approvals bell, the ⌘K chip, and Settings. The mark and the workspace chip are separate `role="menubar"`s so the globe can sit between them without becoming a menu item. Compact chrome keeps mark + globe + chip after the app menus hide.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const WORKSPACES: WorkspacePayload[] = [
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

function MenubarFixture({
  overlay = null,
  live = true,
  scope,
  chip,
  onToggleGlobalScope,
  toggleLocked,
  worktreeSelection,
  listing = MENUBAR_WORKTREE_LISTING,
  updateAvailable = false,
}: {
  overlay?: DesktopOverlay | null;
  live?: boolean;
  scope?: WorkspaceScopeMode;
  chip?: { name: string; monogram: string };
  onToggleGlobalScope?: () => void;
  toggleLocked?: boolean;
  worktreeSelection?: ActiveWorktreeSelection;
  listing?: WorktreesResponse;
  updateAvailable?: boolean;
}) {
  const [shell] = useState(() => (live ? createLiveStoryShell() : createStoryShell()));
  const [active, setActive] = useState<DesktopOverlay | null>(overlay);
  return (
    <OsShellContext.Provider value={shell}>
      <AgentCreateHostProvider openDialog={fn()} openForDuplicate={fn()}>
        <DesktopShell menubar={false} wallpaper="carbon" deskHint>
          <DesktopMenubar
            workspaces={WORKSPACES}
            activeWorkspace={WORKSPACES[0]}
            chip={chip}
            scope={scope}
            toggleLocked={toggleLocked}
            onToggleGlobalScope={onToggleGlobalScope}
            onSelectWorkspace={fn()}
            onAddWorkspace={fn()}
            onNewSession={fn()}
            onOpenPalette={fn()}
            onOpenDesktops={fn()}
            onOpenWorkspaces={fn()}
            onToggleSessions={fn()}
            activeOverlay={active}
            onOverlayOpenChange={(id, open) =>
              setActive(current => (open ? id : current === id ? null : current))
            }
            attention={ATTENTION}
            updateAvailable={updateAvailable}
            worktreesByWorkspace={{ [WORKSPACES[0].id]: listing }}
            userHomeDir="/Users/ada"
            worktreeSelection={worktreeSelection}
            onSelectWorktree={fn()}
            onCreateWorktree={fn()}
          />
        </DesktopShell>
      </AgentCreateHostProvider>
    </OsShellContext.Provider>
  );
}

function remapWorkspaceId<T extends { workspace_id?: string }>(entry: T): T {
  return { ...entry, workspace_id: WORKSPACES[0].id };
}

const MENUBAR_WORKTREE_LISTING: WorktreesResponse = {
  ...worktreeListingFixture,
  worktrees: worktreeListingFixture.worktrees.map(remapWorkspaceId),
};

/** ≤5 rows so discovered and missing survive the nest truncate. */
const MENUBAR_NEST_LISTING: WorktreesResponse = {
  ...worktreeListingFixture,
  worktrees: [worktreeReadyDirtyRunningFixture, worktreePendingFixture, worktreeMissingFixture].map(
    remapWorkspaceId
  ),
  discovered: [discoveredWorktreeFixture],
};

const READY_WORKTREE = MENUBAR_WORKTREE_LISTING.worktrees.find(
  worktree => worktree.state === "ready"
)!;
const MISSING_WORKTREE = MENUBAR_WORKTREE_LISTING.worktrees.find(
  worktree => worktree.state === "missing"
)!;

/** The wired bar at rest, over a live desktop. */
export const Populated: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => <MenubarFixture />,
};

/** Global scope on: the chip reads Global and the menubar toggle is armed. */
export const GlobalScopeOn: Story = {
  args: { workspace: { name: "Global", monogram: "~" } },
  render: () => (
    <MenubarFixture
      chip={{ name: "Global", monogram: "~" }}
      onToggleGlobalScope={fn()}
      scope="global"
      toggleLocked={false}
    />
  ),
};

/** The system menu on the mark: identity plus the settings surfaces. */
export const CompozyMenuOpen: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => <MenubarFixture overlay="compozy-menu" />,
};

/** Navigation grouped exactly like the dock strip. */
export const GoMenuOpen: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => <MenubarFixture overlay="go-menu" />,
};

/** Window commands with a focused tiled window and a visible peer: all enabled. */
export const WindowMenuOpen: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => <MenubarFixture overlay="window-menu" />,
};

/**
 * The same menu on a cold desktop: no live client, so every command is disabled
 * rather than hidden — the shape of the menu never changes under you.
 */
export const WindowMenuUnavailable: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => <MenubarFixture overlay="window-menu" live={false} />,
};

/** Real help: the shortcut reference, the docs, and a support path. */
export const HelpMenuOpen: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => <MenubarFixture overlay="help-menu" />,
};

/**
 * VC-15 — workspace menu with the shared nested worktree projection.
 * Every row stays in the nest — discovered and missing included.
 * Play hovers the git-backed parent so the side submenu is capturable.
 */
export const WorkspaceMenuOpen: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.hover(await body.findByTestId(`os-workspace-option-${WORKSPACES[0].id}`));
    await body.findByTestId(`os-worktree-submenu-${WORKSPACES[0].id}`);
  },
  render: () => <MenubarFixture overlay="workspace-menu" listing={MENUBAR_NEST_LISTING} />,
};

/** Global on: the menu lists project folders with no check and no notices. */
export const WorkspaceMenuOpenWhileGlobal: Story = {
  args: { workspace: { name: "Global", monogram: "~" } },
  render: () => (
    <MenubarFixture
      overlay="workspace-menu"
      chip={{ name: "Global", monogram: "~" }}
      onToggleGlobalScope={fn()}
      scope="global"
      toggleLocked={false}
    />
  ),
};

/** VC-16 — the shell chip names only the workspace at its root scope. */
export const WorkspaceOnlyChip: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => <MenubarFixture />,
};

/** VC-17 — an active worktree is part of the shell's visible scope identity. */
export const WorkspaceAndWorktreeChip: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => (
    <MenubarFixture
      worktreeSelection={{
        selectedWorktreeId: READY_WORKTREE.id,
        activeWorktree: READY_WORKTREE,
        resolved: true,
        fallback: null,
      }}
    />
  ),
};

/**
 * VC-18 — a missing selection falls back to the parent and states why.
 * Play hovers the parent so the missing row and the bar notice are both
 * capturable.
 */
export const MissingWorktreeFallback: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.hover(await body.findByTestId(`os-workspace-option-${WORKSPACES[0].id}`));
    await body.findByTestId(`os-worktree-submenu-${WORKSPACES[0].id}`);
  },
  render: () => (
    <MenubarFixture
      overlay="workspace-menu"
      listing={MENUBAR_NEST_LISTING}
      worktreeSelection={{
        selectedWorktreeId: MISSING_WORKTREE.id,
        activeWorktree: null,
        resolved: true,
        fallback: {
          worktreeId: MISSING_WORKTREE.id,
          name: MISSING_WORKTREE.name,
          reason: "missing",
        },
      }}
    />
  ),
};

/** Presentation-only — no menu owners, so the bar renders as inert chrome. */
export const PresentationOnly: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" }, notifications: 0 },
  render: args => (
    <DesktopShell menubar={false} wallpaper="carbon" deskHint>
      <OsMenuBar {...args} />
    </DesktopShell>
  ),
};

/**
 * Degraded desktop sync — the warning stays non-blocking, names the state in
 * text, and leaves every shell command available.
 */
export const DegradedSync: Story = {
  args: {
    workspace: { name: "compozy", monogram: "CO" },
    status: <OsHydrationStatus hydration="degraded" />,
    notifications: 2,
    onCommandClick: fn(),
    onSettingsClick: fn(),
  },
  render: args => (
    <DesktopShell menubar={false} wallpaper="carbon" deskHint>
      <OsMenuBar {...args} />
    </DesktopShell>
  ),
};

/** Menubar update indicator (S2): one story per state the daemon can report. */
function updateIndicatorStory(snapshot: SettingsUpdateStatus): Story {
  return {
    args: { workspace: { name: "compozy", monogram: "CO" } },
    render: () => <MenubarFixture updateAvailable={settingsUpdateIndicatorAvailable(snapshot)} />,
  };
}

function FocusUpdateIndicatorSetup() {
  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      document.querySelector<HTMLElement>("[data-testid='os-menubar-update']")?.focus();
    });
    return () => cancelAnimationFrame(frame);
  }, []);
  return null;
}

/**
 * An update is offered: a filled accent control joins the trailing cluster
 * before the approvals bell. No count, no dropdown — detail lives in Settings.
 */
export const UpdateIndicatorAvailable: Story = updateIndicatorStory(
  settingsUpdateBothAvailableFixture
);

/** Keyboard focus exposes the indicator's visible focus treatment. */
export const UpdateIndicatorFocused: Story = {
  ...updateIndicatorStory(settingsUpdateBothAvailableFixture),
  render: () => (
    <>
      <MenubarFixture updateAvailable />
      <FocusUpdateIndicatorSetup />
    </>
  ),
};

/** The calm default: nothing is offered, so the indicator is absent from the DOM. */
export const UpdateIndicatorHidden: Story = updateIndicatorStory(settingsUpdateStatusFixture);

/**
 * An operation is live. The offer is over, so the indicator disappears and stays
 * gone through applying, staged, and failed — progress belongs to Settings alone.
 */
export const UpdateIndicatorSuppressed: Story = updateIndicatorStory(settingsUpdateApplyingFixture);
