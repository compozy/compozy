import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import type { WorkspacePayload } from "@/systems/workspace";
import {
  discoveredOnlyWorktreeListingFixture,
  emptyWorktreeListingFixture,
  scrollableWorktreesListingFixture,
  worktreeListingFixture,
} from "@/systems/workspace/mocks";

import type { OsWorkspaceDetailsResult } from "../../hooks/use-workspace-details";
import { OsWorkspacesOverview } from "../os-workspaces-overview";

function workspace(id: string, name: string, rootDir: string): WorkspacePayload {
  return {
    id,
    name,
    root_dir: rootDir,
    add_dirs: [],
    created_at: "2026-04-17T08:00:00Z",
    updated_at: "2026-04-17T17:30:00Z",
  };
}

const GIT_WORKSPACE = workspace("ws_launch_hq", "launch-hq", "/dev/launch-hq");
const PLAIN_WORKSPACE = workspace("ws_branas_site", "branas-site", "/dev/branas-site");

function details(workspaces: WorkspacePayload[]): OsWorkspaceDetailsResult {
  return {
    byWorkspaceId: new Map(
      workspaces.map(entry => [
        entry.id,
        { status: "ready" as const, agentCount: 3, sessionCount: 5, agentNames: ["atlas"] },
      ])
    ),
    totalAgents: 3,
  } as OsWorkspaceDetailsResult;
}

const meta: Meta<typeof OsWorkspacesOverview> = {
  title: "systems/os/components/OsWorkspacesOverview",
  component: OsWorkspacesOverview,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Workspace overview cards with a side worktree submenu. Hover or focus the Worktrees control to open the same nest the menubar uses; a click on the card still enters the workspace.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const openLaunchHqWorktrees: Story["play"] = async ({ canvasElement }) => {
  const body = within(canvasElement.ownerDocument.body);
  await userEvent.hover(
    await body.findByTestId(`os-workspace-worktrees-toggle-${GIT_WORKSPACE.id}`)
  );
  await body.findByTestId(`os-worktree-submenu-${GIT_WORKSPACE.id}`);
};

/** VC-19 — a workspace card with its worktree count and a hover side submenu. */
export const WithWorktrees: Story = {
  args: {},
  tags: ["play-fn"],
  play: openLaunchHqWorktrees,
  render: () => (
    <StorySurface>
      <OsWorkspacesOverview
        open
        onOpenChange={fn()}
        workspaces={[GIT_WORKSPACE]}
        activeWorkspaceId={GIT_WORKSPACE.id}
        onSelectWorkspace={fn()}
        onNewWorkspace={fn()}
        details={details([GIT_WORKSPACE])}
        presentation="floating"
        reducedMotion
        worktreesByWorkspace={{ [GIT_WORKSPACE.id]: worktreeListingFixture }}
        userHomeDir="/Users/ada"
        onSelectWorktree={fn()}
        onCreateWorktree={fn()}
      />
    </StorySurface>
  ),
};

export const ScrollableWorktrees: Story = {
  args: {},
  tags: ["play-fn"],
  play: openLaunchHqWorktrees,
  render: () => (
    <StorySurface>
      <OsWorkspacesOverview
        open
        onOpenChange={fn()}
        workspaces={[GIT_WORKSPACE]}
        activeWorkspaceId={GIT_WORKSPACE.id}
        onSelectWorkspace={fn()}
        onNewWorkspace={fn()}
        details={details([GIT_WORKSPACE])}
        presentation="floating"
        reducedMotion
        worktreesByWorkspace={{ [GIT_WORKSPACE.id]: scrollableWorktreesListingFixture }}
        userHomeDir="/Users/ada"
        onSelectWorktree={fn()}
        onCreateWorktree={fn()}
      />
    </StorySurface>
  ),
};

/** VC-20 — git-backed with none created: no count; the chevron still opens New worktree. */
export const WithoutWorktrees: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <OsWorkspacesOverview
        open
        onOpenChange={fn()}
        workspaces={[PLAIN_WORKSPACE]}
        activeWorkspaceId={PLAIN_WORKSPACE.id}
        onSelectWorkspace={fn()}
        onNewWorkspace={fn()}
        details={details([PLAIN_WORKSPACE])}
        presentation="floating"
        reducedMotion
        worktreesByWorkspace={{ [PLAIN_WORKSPACE.id]: emptyWorktreeListingFixture }}
        userHomeDir="/Users/ada"
        onSelectWorktree={fn()}
        onCreateWorktree={fn()}
      />
    </StorySurface>
  ),
};

/** VC-21 — a discovered external keeps its foreign path and enters no count. */
export const DiscoveredExternal: Story = {
  args: {},
  tags: ["play-fn"],
  play: openLaunchHqWorktrees,
  render: () => (
    <StorySurface>
      <OsWorkspacesOverview
        open
        onOpenChange={fn()}
        workspaces={[GIT_WORKSPACE]}
        activeWorkspaceId={GIT_WORKSPACE.id}
        onSelectWorkspace={fn()}
        onNewWorkspace={fn()}
        details={details([GIT_WORKSPACE])}
        presentation="floating"
        reducedMotion
        worktreesByWorkspace={{ [GIT_WORKSPACE.id]: discoveredOnlyWorktreeListingFixture }}
        userHomeDir="/Users/ada"
        onSelectWorktree={fn()}
        onCreateWorktree={fn()}
      />
    </StorySurface>
  ),
};
