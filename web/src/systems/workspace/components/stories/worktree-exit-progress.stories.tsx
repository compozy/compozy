import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import {
  exitProgressAdvancingFixture,
  exitProgressAnnouncedFixture,
  exitProgressFailureFixture,
  exitProgressHookFailureFixture,
  exitProgressSuccessFixture,
} from "../../mocks/worktree-exit-progress-fixtures";
import { WorktreeExitProgress } from "../worktree-exit-progress";

const meta: Meta<typeof WorktreeExitProgress> = {
  title: "systems/workspace/components/WorktreeExitProgress",
  component: WorktreeExitProgress,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "One updating report for one exit operation. Every phase is announced on the first frame, hook output streams in arrival order and stays visible after the phase ends, and a finished operation offers exactly one next step — the one the daemon named.",
      },
    },
  },
  render: args => (
    <StorySurface className="flex items-center justify-center p-6">
      <WorktreeExitProgress {...args} />
    </StorySurface>
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-58 — the whole sequence is visible before any of it has run. */
export const PhasesAnnounced: Story = {
  args: { progress: exitProgressAnnouncedFixture, onCancel: fn(), onDismiss: fn() },
};

/** VC-59 — advancing, with the hook's own output as it prints. stderr is marked. */
export const AdvancingWithHookStream: Story = {
  args: { progress: exitProgressAdvancingFixture, onCancel: fn(), onDismiss: fn() },
};

/** VC-60 — done: past-tense labels, the skipped push reason, and one CTA. */
export const SuccessWithSkipReasonAndCta: Story = {
  args: { progress: exitProgressSuccessFixture, onCta: fn(), onDismiss: fn() },
};

/** VC-61 — the failure belongs to the phase that produced it, with that output. */
export const MidChainFailure: Story = {
  args: { progress: exitProgressFailureFixture, onDismiss: fn() },
};

/**
 * VC-51 — a hook refused the commit. Streamed hook output stays on this report
 * because the commit dialog has already closed.
 */
export const HookFailure: Story = {
  args: { progress: exitProgressHookFailureFixture, onDismiss: fn() },
};
