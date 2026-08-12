import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { AgentCreateHostProvider } from "@/systems/agent";
import type {
  ActiveWorktreeSelection,
  WorkspacePayload,
  WorkspaceScopeMode,
} from "@/systems/workspace";
import { worktreeListingFixture } from "@/systems/workspace/mocks";

import { OsShellContext } from "../../contexts/os-shell-context";
import type { OsAttentionModel } from "../../hooks/use-os-attention";
import type { DesktopOverlay } from "../../hooks/use-desktop-overlays";
import { DesktopMenubar } from "../desktop-menubar";
import { OsMenuBar } from "../os-menubar";
import { OsHydrationStatus } from "../os-hydration-status";
import { createLiveStoryShell, createStoryShell } from "./_shell-fixture";
import { DesktopShell } from "./_desktop";

const meta: Meta<typeof OsMenuBar> = {
  title: "systems/os/components/OsMenuBar",
  component: OsMenuBar,
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
  rows: [],
  sessions: [],
  archivedSessions: [],
  archivedSessionsTotal: 0,
  sessionsDisconnected: false,
  tasksDisconnected: false,
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
}: {
  overlay?: DesktopOverlay | null;
  live?: boolean;
  scope?: WorkspaceScopeMode;
  chip?: { name: string; monogram: string };
  onToggleGlobalScope?: () => void;
  toggleLocked?: boolean;
  worktreeSelection?: ActiveWorktreeSelection;
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
            worktreesByWorkspace={{ [WORKSPACES[0].id]: MENUBAR_WORKTREE_LISTING }}
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

const MENUBAR_WORKTREE_LISTING = {
  ...worktreeListingFixture,
  worktrees: worktreeListingFixture.worktrees.map(worktree => ({
    ...worktree,
    workspace_id: WORKSPACES[0].id,
  })),
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

/** VC-15 — workspace menu with the shared nested worktree projection. */
export const WorkspaceMenuOpen: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => <MenubarFixture overlay="workspace-menu" />,
};

/** Global on: the menu lists project folders with no check, plus the scope notice. */
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
        fallback: null,
      }}
    />
  ),
};

/** VC-18 — a missing selection falls back to the parent and states why. */
export const MissingWorktreeFallback: Story = {
  args: { workspace: { name: "compozy", monogram: "CO" } },
  render: () => (
    <MenubarFixture
      worktreeSelection={{
        selectedWorktreeId: MISSING_WORKTREE.id,
        activeWorktree: null,
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
