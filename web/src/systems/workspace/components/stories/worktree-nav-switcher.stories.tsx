import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn } from "storybook/test";

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
    <CenteredSurface className="w-[420px] border border-line bg-canvas-soft p-0">
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
    </CenteredSurface>
  );
}

const meta: Meta = {
  title: "systems/workspace/components/WorktreeNavSwitcher",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The command switcher with worktrees nested under their parent workspace. Worktree rows are real command items, so cmdk's own filtering and arrow-key order apply.",
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
  render: () => (
    <Harness
      workspaces={[GIT_WORKSPACE]}
      worktreesByWorkspace={{ [GIT_WORKSPACE.id]: manyWorktreesListingFixture }}
    />
  ),
};

/**
 * VC-12 — the parent aggregate the switcher actually keeps. Production has no
 * fold: the git-backed nest stays open beside the quiet "N active" signal
 * (authorized delta vs artboard §08 — see task-06-authorized-deltas.md).
 * Zero activity renders nothing, never "0".
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
  render: () => (
    <Harness
      workspaces={[GIT_WORKSPACE]}
      worktreesByWorkspace={{ [GIT_WORKSPACE.id]: discoveredOnlyWorktreeListingFixture }}
    />
  ),
};
