import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import type { WorktreeDetailModel } from "../../hooks/use-worktree-detail-context";
import { worktreeReadyDirtyRunningFixture } from "../../mocks/worktree-fixtures";
import { toWorktreeExitLadder, type WorktreeExitLadder } from "../../lib/worktree-exit-ladder";
import {
  exitPlanAwaitingInputFixture,
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
  worktreeStatusDivergedFixture,
  worktreeStatusUnknownFixture,
} from "../../mocks/worktree-exit-fixtures";
import { WorktreeDetailDialog } from "../worktree-detail-dialog";
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

/** A ready detail model. Progress and dialogs stay closed — the hero is the subject. */
function detailModel(
  overrides: Partial<WorktreeDetailModel> & { ladder: WorktreeExitLadder }
): WorktreeDetailModel {
  return {
    forgeStatus: undefined,
    isRunning: false,
    onAction: fn(),
    onDismissProgress: fn(),
    onRemove: fn(),
    progress: undefined,
    sessionTitle: "Fix payment retries",
    state: "ready",
    status: worktreeStatusDirtyFixture.status,
    worktree: worktreeReadyDirtyRunningFixture,
    ...overrides,
  };
}

function control(plan: typeof exitPlanCommitFixture) {
  return (
    <StorySurface className="p-6">
      <WorktreeExitControl ladder={toWorktreeExitLadder(plan)} onAction={() => {}} />
    </StorySurface>
  );
}

/**
 * VC-36 — the worktree context at rest: identity, the five git facts, and the
 * one assisted way out, with the bound agent awaiting input (the state in which
 * exit actions unlock).
 */
export const DetailContextHero: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanAwaitingInputFixture), onAction: () => {} },
  parameters: { layout: "fullscreen" },
  render: () => (
    <WorktreeDetailDialog
      model={detailModel({
        ladder: toWorktreeExitLadder(exitPlanAwaitingInputFixture),
        worktree: { ...worktreeReadyDirtyRunningFixture, agent_activity: "awaiting-input" },
      })}
      onOpenChange={() => {}}
      open
    />
  ),
};

/** VC-37 — a dirty tree: Commit leads. */
export const CommitPosition: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanCommitFixture), onAction: () => {} },
  render: () => control(exitPlanCommitFixture),
};

/** VC-38 — Push leads. Publish variant: the payload's own label says it creates the branch. */
export const PushPosition: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanPushFixture), onAction: () => {} },
  render: () => control(exitPlanPushFixture),
};

/** VC-39 — the request step leads, in the provider's own vocabulary. */
export const OpenRequestPosition: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanOpenPrFixture), onAction: () => {} },
  render: () => control(exitPlanOpenPrFixture),
};

/** VC-40 — a request already exists, so viewing it is the step. */
export const ViewRequestPosition: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanViewPrFixture), onAction: () => {} },
  render: () => control(exitPlanViewPrFixture),
};

/** VC-41 — the menu open: every row carries the daemon's own reason for its state. */
export const ExitMenuBlockedReasons: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanCommitFixture), onAction: () => {} },
  render: () => control(exitPlanCommitFixture),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByRole("button", { name: "Git actions" }));
  },
};

/** VC-42 — one cause blocks every row, in the daemon's words. */
export const PausedWhileSessionRunning: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanPausedFixture), onAction: () => {} },
  render: () => control(exitPlanPausedFixture),
};

/** VC-43 — the read failed, so unknowns are em-dashes and the whole control is blocked. */
export const StatusUnreadable: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanStatusUnreadableFixture), onAction: () => {} },
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

/** VC-44 — diverged from upstream: the push-class row states the cause. */
export const DivergedFromUpstream: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanDivergedFixture), onAction: () => {} },
  render: () => (
    <StorySurface className="flex flex-col gap-3 p-6">
      <WorktreeStatusStrip status={worktreeStatusDivergedFixture.status} />
      <WorktreeExitControl
        ladder={toWorktreeExitLadder(exitPlanDivergedFixture)}
        onAction={() => {}}
      />
    </StorySurface>
  ),
};

/** VC-45 — no remote: the ladder is commit-only. */
export const NoRemote: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanNoRemoteFixture), onAction: () => {} },
  render: () => control(exitPlanNoRemoteFixture),
};

/**
 * VC-46 — zero credential: no forge serves the remote, so the strip drops its PR
 * cell entirely and the compare link is the whole request path. Absent, never
 * disabled as a forge row.
 */
export const ZeroCredential: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanZeroCredentialFixture), onAction: () => {} },
  render: () => (
    <StorySurface className="flex flex-col gap-3 p-6">
      <WorktreeStatusStrip status={worktreeStatusDirtyFixture.status} />
      <WorktreeExitControl
        ladder={toWorktreeExitLadder(exitPlanZeroCredentialFixture)}
        onAction={() => {}}
      />
    </StorySurface>
  ),
};

/** Dirty git facts for one worktree: porcelain counts, never inferred zeros. */
export const StatusStripDirty: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanCommitFixture), onAction: () => {} },
  render: () => (
    <StorySurface className="p-6">
      <WorktreeStatusStrip
        forge={exitPlanCommitFixture.forge ?? undefined}
        status={worktreeStatusDirtyFixture.status}
      />
    </StorySurface>
  ),
};

/**
 * The safe and blocked verdicts side by side, as the exit surface pairs them.
 * The per-state capture targets (VC-62..VC-67) live in
 * `worktree-merged-cleanup.stories.tsx`.
 */
export const MergedEvidence: Story = {
  args: { ladder: toWorktreeExitLadder(exitPlanMergedFixture), onAction: () => {} },
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
