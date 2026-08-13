import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import {
  exitPlanCleanupBlockedFixture,
  exitPlanMergedFixture,
} from "../../mocks/worktree-exit-fixtures";
import {
  cleanupClosedWithoutMergeFixture,
  cleanupLocalSafeFixture,
  cleanupRemoteDowngradeFixture,
  cleanupStaleRemoteFixture,
} from "../../mocks/worktree-exit-progress-fixtures";
import { WorktreeMergedEvidence } from "../worktree-merged-evidence";

const meta: Meta<typeof WorktreeMergedEvidence> = {
  title: "systems/workspace/components/WorktreeMergedCleanup",
  component: WorktreeMergedEvidence,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Why this worktree is — or is not — safe to clean up. The verdict and its blocker are the daemon's; a blocker suppresses the action entirely rather than disabling it, and a downgraded verdict reads as information, because a branch still on the remote is a fact, not a failure.",
      },
    },
  },
  render: args => (
    <StorySurface className="flex w-[560px] items-center p-6">
      <WorktreeMergedEvidence {...args} />
    </StorySurface>
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-62 — merged on the forge: "Merged on github". */
export const ForgeMerged: Story = {
  args: { cleanup: exitPlanMergedFixture.cleanup, onCleanUp: fn() },
};

/** VC-63 — closed without merging: forge_state plus the local unique-commits blocker. */
export const ClosedWithoutMerge: Story = {
  args: { cleanup: cleanupClosedWithoutMergeFixture, onCleanUp: fn() },
};

/** VC-64 — no forge in the picture: "No commits unique to this branch." */
export const LocalSafeToClean: Story = {
  args: { cleanup: cleanupLocalSafeFixture, onCleanUp: fn() },
};

/** VC-65 — unique commits are already on a remote branch, so the verdict downgrades. */
export const RemoteExistsDowngrade: Story = {
  args: { cleanup: cleanupRemoteDowngradeFixture, onCleanUp: fn() },
};

/** VC-66 — "3 commits exist nowhere else." suppresses Clean up. */
export const NotSafeBlocker: Story = {
  args: { cleanup: exitPlanCleanupBlockedFixture.cleanup, onCleanUp: fn() },
};

/** VC-67 — the remote read has aged, so the evidence carries when it was true. */
export const StaleRemoteMarker: Story = {
  args: { cleanup: cleanupStaleRemoteFixture, onCleanUp: fn(), staleLabel: "12 minutes ago" },
};
