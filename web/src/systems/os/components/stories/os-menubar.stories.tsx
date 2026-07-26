import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { AgentCreateHostProvider } from "@/systems/agent";
import type { WorkspacePayload } from "@/systems/workspace";

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
          'The desktop menubar: the AGH system menu, the workspace switcher, the static Session / Go / Window / Help set, the approvals bell, the ⌘K chip, and Settings. The mark, the workspace chip, and every app menu sit in one `role="menubar"`, so arrow keys traverse the whole bar and hovering a sibling switches the open menu.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const WORKSPACES: WorkspacePayload[] = [
  {
    id: "workspace-agh",
    name: "agh",
    root_dir: "/workspace/agh",
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
  sessionsDisconnected: false,
  tasksDisconnected: false,
  loading: false,
};

function MenubarFixture({
  overlay = null,
  live = true,
}: {
  overlay?: DesktopOverlay | null;
  live?: boolean;
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
          />
        </DesktopShell>
      </AgentCreateHostProvider>
    </OsShellContext.Provider>
  );
}

/** The wired bar at rest, over a live desktop. */
export const Populated: Story = {
  args: { workspace: { name: "agh", monogram: "AG" } },
  render: () => <MenubarFixture />,
};

/** The system menu on the mark: identity plus the settings surfaces. */
export const AghMenuOpen: Story = {
  args: { workspace: { name: "agh", monogram: "AG" } },
  render: () => <MenubarFixture overlay="agh-menu" />,
};

/** Navigation grouped exactly like the dock strip. */
export const GoMenuOpen: Story = {
  args: { workspace: { name: "agh", monogram: "AG" } },
  render: () => <MenubarFixture overlay="go-menu" />,
};

/** Window commands with a focused tiled window and a visible peer: all enabled. */
export const WindowMenuOpen: Story = {
  args: { workspace: { name: "agh", monogram: "AG" } },
  render: () => <MenubarFixture overlay="window-menu" />,
};

/**
 * The same menu on a cold desktop: no live client, so every command is disabled
 * rather than hidden — the shape of the menu never changes under you.
 */
export const WindowMenuUnavailable: Story = {
  args: { workspace: { name: "agh", monogram: "AG" } },
  render: () => <MenubarFixture overlay="window-menu" live={false} />,
};

/** Real help: the shortcut reference, the docs, and a support path. */
export const HelpMenuOpen: Story = {
  args: { workspace: { name: "agh", monogram: "AG" } },
  render: () => <MenubarFixture overlay="help-menu" />,
};

/** Workspace switcher with the bound set and the overview/creation paths. */
export const WorkspaceMenuOpen: Story = {
  args: { workspace: { name: "agh", monogram: "AG" } },
  render: () => <MenubarFixture overlay="workspace-menu" />,
};

/** Presentation-only — no menu owners, so the bar renders as inert chrome. */
export const PresentationOnly: Story = {
  args: { workspace: { name: "agh", monogram: "AG" }, notifications: 0 },
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
    workspace: { name: "agh", monogram: "AG" },
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
