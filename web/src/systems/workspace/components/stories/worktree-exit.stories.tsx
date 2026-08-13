import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import { toWorktreeExitLadder } from "../../lib/worktree-exit-ladder";
import {
  exitPlanCleanupBlockedFixture,
  exitPlanCommitFixture,
  exitPlanDivergedFixture,
  exitPlanMergedFixture,
  exitPlanNoRemoteFixture,
  exitPlanOpenPrFixture,
  exitPlanPausedFixture,
  exitPlanPushFixture,
  exitPlanStatusUnreadableFixture,
  exitPlanViewPrFixture,
  exitPlanZeroCredentialFixture,
  worktreeStatusDirtyFixture,
  worktreeStatusUnknownFixture,
} from "../../mocks/worktree-exit-fixtures";
import { WorktreeExitControl } from "../worktree-exit-control";
import { WorktreeMergedEvidence } from "../worktree-merged-evidence";
import { WorktreeStatusStrip } from "../worktree-status-strip";

const meta: Meta<typeof WorktreeExitControl> = {
  title: "systems/workspace/components/WorktreeExitControl",
  component: WorktreeExitControl,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The assisted exit, rendered exactly as the daemon computed it. Which action leads, which are blocked and why, and which exist at all are the plan's decisions — every state below is one payload, not one branch of client logic.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function control(plan: typeof exitPlanCommitFixture) {
  return (
    <StorySurface className="p-6">
      <WorktreeExitControl ladder={toWorktreeExitLadder(plan)} onAction={() => {}} />
    </StorySurface>
  );
}

export const CommitPosition: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanCommitFixture), onAction: () => {} },
  render: () => control(exitPlanCommitFixture),
};

/** Publish variant: the payload's own label already says it creates the branch. */
export const PushPosition: Story = {
  args: CommitPosition.args,
  render: () => control(exitPlanPushFixture),
};

export const OpenRequestPosition: Story = {
  args: CommitPosition.args,
  render: () => control(exitPlanOpenPrFixture),
};

export const ViewRequestPosition: Story = {
  args: CommitPosition.args,
  render: () => control(exitPlanViewPrFixture),
};

/** One cause blocks every row, in the daemon's words. */
export const PausedWhileSessionRunning: Story = {
  args: CommitPosition.args,
  render: () => control(exitPlanPausedFixture),
};

export const StatusUnreadable: Story = {
  args: CommitPosition.args,
  render: () => (
    <StorySurface className="flex flex-col gap-3 p-6">
      <WorktreeStatusStrip status={worktreeStatusUnknownFixture.status} />
      <WorktreeExitControl
        ladder={toWorktreeExitLadder(exitPlanStatusUnreadableFixture)}
        onAction={() => {}}
      />
    </StorySurface>
  ),
};

export const DivergedFromUpstream: Story = {
  args: CommitPosition.args,
  render: () => control(exitPlanDivergedFixture),
};

/** No remote: the push-class row states why; no PR row exists at all. */
export const NoRemote: Story = {
  args: CommitPosition.args,
  render: () => control(exitPlanNoRemoteFixture),
};

/** Zero credential: PR affordances are absent, and the compare link is the step. */
export const ZeroCredential: Story = {
  args: CommitPosition.args,
  render: () => control(exitPlanZeroCredentialFixture),
};

export const StatusStripDirty: Story = {
  args: CommitPosition.args,
  render: () => (
    <StorySurface className="p-6">
      <WorktreeStatusStrip
        forge={exitPlanCommitFixture.forge ?? undefined}
        status={worktreeStatusDirtyFixture.status}
      />
    </StorySurface>
  ),
};

export const MergedEvidence: Story = {
  args: CommitPosition.args,
  render: () => (
    <StorySurface className="flex flex-col gap-3 p-6">
      <WorktreeMergedEvidence cleanup={exitPlanMergedFixture.cleanup} onCleanUp={() => {}} />
      <WorktreeMergedEvidence
        cleanup={exitPlanCleanupBlockedFixture.cleanup}
        onCleanUp={() => {}}
      />
    </StorySurface>
  ),
};
