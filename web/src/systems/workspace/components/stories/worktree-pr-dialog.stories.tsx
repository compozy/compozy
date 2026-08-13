import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import { toWorktreeExitLadder } from "../../lib/worktree-exit-ladder";
import {
  exitPlanOpenPrFixture,
  exitPlanPrBlockedFixture,
  exitPlanTemplateAmbiguousFixture,
  exitPlanViewPrFixture,
  exitPlanZeroCredentialCleanFixture,
  prPrefillFixture,
} from "../../mocks/worktree-exit-fixtures";
import { WorktreePrDialog } from "../worktree-pr-dialog";

const WORKTREE_NAME = "payments-retry";
const BRANCH = "pedro/payments-retry";

const meta: Meta<typeof WorktreePrDialog> = {
  title: "systems/workspace/components/WorktreePrDialog",
  component: WorktreePrDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The pull-request step. Every row exists because the plan offers the action behind it: with no forge serving the remote the create path is absent entirely, and template resolution happens server-side, so the browser never prefills on its own.",
      },
    },
  },
  render: args => (
    <StorySurface>
      <WorktreePrDialog {...args} />
    </StorySurface>
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

const baseArgs = {
  open: true,
  onOpenChange: fn(),
  onSubmit: fn(),
  worktreeName: WORKTREE_NAME,
  branch: BRANCH,
  ladder: toWorktreeExitLadder(exitPlanOpenPrFixture),
  prefill: prPrefillFixture,
};

/** VC-52 — the resolved base, editable title and description, daemon-supplied prefill. */
export const BaseResolvedWithPrefill: Story = {
  args: baseArgs,
};

/**
 * VC-53 — the provider supports drafts, so draft is a peer row rather than a
 * checkbox hidden inside the primary action.
 */
export const DraftPeerRow: Story = { args: baseArgs };

/** VC-54 — a request is already open: viewing it is the only row left. */
export const OpenedExisting: Story = {
  args: { ...baseArgs, ladder: toWorktreeExitLadder(exitPlanViewPrFixture) },
};

/**
 * VC-55 — no forge serves the remote: the inputs are absent, not disabled, and
 * the compare link the daemon resolved is the whole step.
 */
export const ZeroCredentialBrowserOnly: Story = {
  args: { ...baseArgs, ladder: toWorktreeExitLadder(exitPlanZeroCredentialCleanFixture) },
};

/**
 * VC-56 — more than one template resolves, so the body stays empty. The title
 * is still the branch; the dialog does not claim a template.
 */
export const TemplateAmbiguousQuietDegrade: Story = {
  args: {
    ...baseArgs,
    ladder: toWorktreeExitLadder(exitPlanTemplateAmbiguousFixture),
    prefill: exitPlanTemplateAmbiguousFixture.pr_prefill ?? undefined,
  },
};

/** VC-57 — the step exists but cannot run; the daemon's reason is the whole state. */
export const NoChangesBlocked: Story = {
  args: { ...baseArgs, ladder: toWorktreeExitLadder(exitPlanPrBlockedFixture) },
};
