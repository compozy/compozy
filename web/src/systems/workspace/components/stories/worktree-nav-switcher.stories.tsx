import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn, userEvent, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import {
  discoveredOnlyWorktreeListingFixture,
  emptyWorktreeListingFixture,
  manyWorktreesListingFixture,
  nonGitWorktreeListingFixture,
  worktreeListingFixture,
  worktreeMissingFixture,
  worktreePendingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import type { WorktreeListingByWorkspace } from "../../lib/workspace-tree";
import { WorkspaceCommandSelect } from "../workspace-command-select";

const GIT_WORKSPACE = { id: "ws_launch_hq", name: "launch-hq", root_dir: "/dev/launch-hq" };
const PLAIN_WORKSPACE = { id: "ws_branas_site", name: "branas-site", root_dir: "/dev/branas-site" };
const NON_GIT_WORKSPACE = { id: "ws_notes", name: "notes", root_dir: "/dev/notes" };

const ALL_WORKSPACES = [GIT_WORKSPACE, PLAIN_WORKSPACE, NON_GIT_WORKSPACE];

async function openWorktreeSubmenu(canvasElement: HTMLElement, workspaceId: string) {
  const body = within(canvasElement.ownerDocument.body);
  await userEvent.hover(await body.findByTestId(`workspace-command-item-${workspaceId}`));
  await body.findByTestId(`workspace-command-worktree-submenu-${workspaceId}`);
}

function Harness({
  worktreesByWorkspace,
  workspaces = ALL_WORKSPACES,
  selectedWorktreeId,
}: {
  worktreesByWorkspace: WorktreeListingByWorkspace;
  workspaces?: typeof ALL_WORKSPACES;
  selectedWorktreeId?: string | null;
}) {
  const [value, setValue] = useState<string | null>(workspaces[0]?.id ?? null);
  return (
    <CenteredSurface className="items-start">
      <div className="w-[420px] border border-line bg-canvas-soft">
        <WorkspaceCommandSelect
          userHomeDir="/Users/ada"
          workspaces={workspaces}
          value={value}
          onChange={setValue}
          open
          onOpenChange={fn()}
          worktreesByWorkspace={worktreesByWorkspace}
          selectedWorktreeId={selectedWorktreeId}
          onSelectWorktree={fn()}
          onCreateWorktree={fn()}
          onShowAllWorktrees={fn()}
          onAddWorkspace={fn()}
        />
      </div>
    </CenteredSurface>
  );
}

const meta: Meta<typeof WorkspaceCommandSelect> = {
  title: "systems/workspace/components/WorktreeNavSwitcher",
  component: WorkspaceCommandSelect,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The command switcher with worktrees in a side submenu. Hover or ArrowRight opens the same nest the menubar uses; a pointer click on the workspace selects it and closes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-06 — a workspace that is not git-backed gets no worktree affordance. */
export const NonGitWorkspace: Story = {
  args: {},
  render: () => (
    <Harness
      workspaces={[NON_GIT_WORKSPACE]}
      worktreesByWorkspace={{ [NON_GIT_WORKSPACE.id]: nonGitWorktreeListingFixture }}
    />
  ),
};

/** VC-07 — git-backed with zero worktrees: no group noise, just the create row. */
export const ZeroWorktrees: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    await openWorktreeSubmenu(canvasElement, PLAIN_WORKSPACE.id);
  },
  render: () => (
    <Harness
      workspaces={[PLAIN_WORKSPACE]}
      worktreesByWorkspace={{ [PLAIN_WORKSPACE.id]: emptyWorktreeListingFixture }}
    />
  ),
};

/** VC-08 — the expanded nest with adopted worktrees. */
export const ExpandedNest: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    await openWorktreeSubmenu(canvasElement, GIT_WORKSPACE.id);
  },
  render: () => (
    <Harness
      workspaces={[GIT_WORKSPACE]}
      worktreesByWorkspace={{
        [GIT_WORKSPACE.id]: {
          ...worktreeListingFixture,
          worktrees: worktreeListingFixture.worktrees.slice(0, 3),
          discovered: [],
        },
      }}
    />
  ),
};

/** VC-09 — a discovered checkout mixed into the nest, selectable to adopt. */
export const DiscoveredMixedIn: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    await openWorktreeSubmenu(canvasElement, GIT_WORKSPACE.id);
  },
  render: () => (
    <Harness
      workspaces={[GIT_WORKSPACE]}
      worktreesByWorkspace={{
        [GIT_WORKSPACE.id]: {
          ...worktreeListingFixture,
          worktrees: [worktreeReadyDirtyRunningFixture],
        },
      }}
    />
  ),
};

/** VC-10 — pending and missing rows stay inert with their reason in a lane. */
export const InertRowsWithReasons: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    await openWorktreeSubmenu(canvasElement, GIT_WORKSPACE.id);
  },
  render: () => (
    <Harness
      workspaces={[GIT_WORKSPACE]}
      worktreesByWorkspace={{
        [GIT_WORKSPACE.id]: {
          ...worktreeListingFixture,
          worktrees: [
            worktreeReadyDirtyRunningFixture,
            worktreePendingFixture,
            worktreeMissingFixture,
          ],
          discovered: [],
        },
      }}
    />
  ),
};

/** VC-11 — past five entries the nest truncates behind an adopted-only count. */
export const TruncationWithOverflow: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    await openWorktreeSubmenu(canvasElement, GIT_WORKSPACE.id);
  },
  render: () => (
    <Harness
      workspaces={[GIT_WORKSPACE]}
      worktreesByWorkspace={{ [GIT_WORKSPACE.id]: manyWorktreesListingFixture }}
    />
  ),
};

/**
 * VC-12 — collapsed parents keep the quiet "N active" signal. Git-backed
 * workspaces start folded; the nest opens on hover. Zero activity
 * renders nothing, never "0".
 */
export const CollapsedAggregate: Story = {
  args: {},
  render: () => (
    <Harness
      workspaces={ALL_WORKSPACES}
      worktreesByWorkspace={{
        [GIT_WORKSPACE.id]: worktreeListingFixture,
        [PLAIN_WORKSPACE.id]: emptyWorktreeListingFixture,
        [NON_GIT_WORKSPACE.id]: nonGitWorktreeListingFixture,
      }}
      selectedWorktreeId={worktreeReadyDirtyRunningFixture.id}
    />
  ),
};

/** A first scan where nothing has been adopted yet: no count, chips carry it. */
export const DiscoveredOnly: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    await openWorktreeSubmenu(canvasElement, GIT_WORKSPACE.id);
  },
  render: () => (
    <Harness
      workspaces={[GIT_WORKSPACE]}
      worktreesByWorkspace={{ [GIT_WORKSPACE.id]: discoveredOnlyWorktreeListingFixture }}
    />
  ),
};
