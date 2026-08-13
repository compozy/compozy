import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import { toWorktreeExitLadder } from "../../lib/worktree-exit-ladder";
import {
  exitPlanCommitFixture,
  exitPlanNothingToCommitFixture,
} from "../../mocks/worktree-exit-fixtures";
import { WorktreeCommitDialog } from "../worktree-commit-dialog";

const WORKTREE_NAME = "payments-retry";
const BRANCH = "pedro/payments-retry";

const COMMIT_LADDER = toWorktreeExitLadder(exitPlanCommitFixture);
const COMMIT_ROWS = COMMIT_LADDER.rows.filter(
  row => row.action === "commit" || row.action === "commit_push"
);
const EMPTY_LADDER = toWorktreeExitLadder(exitPlanNothingToCommitFixture);
const EMPTY_COMMIT_ROWS = EMPTY_LADDER.rows.filter(
  row => row.action === "commit" || row.action === "commit_push"
);

const meta: Meta<typeof WorktreeCommitDialog> = {
  title: "systems/workspace/components/WorktreeCommitDialog",
  component: WorktreeCommitDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The commit step shows the porcelain scope the daemon would stage. Leaving the message blank uses the daemon's default; there is no generation path. A refused hook is reported on the progress surface because the dialog closes when the action starts.",
      },
    },
  },
  render: args => (
    <StorySurface>
      <WorktreeCommitDialog {...args} />
    </StorySurface>
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

const baseArgs = {
  open: true,
  onOpenChange: fn(),
  onCommit: fn(),
  worktreeName: WORKTREE_NAME,
  branch: BRANCH,
  scope: COMMIT_LADDER.commitScope,
  actions: COMMIT_ROWS,
};

/** VC-47 — porcelain dirty total as the file count, with untracked additions listed by name. */
export const ScopeCountsAndUntracked: Story = { args: baseArgs };

/**
 * VC-48 — the empty message and its placeholder.
 *
 * There is no generation path in this release, so the placeholder says "default",
 * not "generated" — a promise against a hardcoded constant would be a UI lie.
 * The dialog opens with an empty message, so this is the same render as VC-47
 * under a different subject; both rows keep their own capture target.
 */
export const EmptyMessagePlaceholder: Story = { args: baseArgs };

/** VC-49 — a session is bound, so the agent can be asked to draft the message. */
export const AgentStagedPrompt: Story = {
  args: { ...baseArgs, onStagePrompt: fn() },
};

/** VC-50 — nothing to commit: the daemon's reason replaces the scope block. */
export const NothingToCommit: Story = {
  args: {
    ...baseArgs,
    scope: EMPTY_LADDER.commitScope,
    actions: EMPTY_COMMIT_ROWS,
    blockedReason: "No uncommitted changes to commit.",
  },
};
