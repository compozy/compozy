import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import { SessionEnvironmentChip } from "../session-environment-chip";

const meta: Meta<typeof SessionEnvironmentChip> = {
  title: "systems/session/components/SessionEnvironmentChip",
  component: SessionEnvironmentChip,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "States where a live session runs. A session's worktree binding is immutable, so the chip is a statement of fact rather than a picker — the only interactive variant leads to a fork, and only when the daemon reports `/worktree` as available.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const RootBinding: Story = {
  args: { state: "root", label: "compozy", mono: "root" },
};

/** Bound, with the daemon refusing the fork right now and saying why. */
export const WorktreeBindingForkUnavailable: Story = {
  args: {
    state: "worktree",
    label: "payments-retry",
    forkUnavailableReason: "Wait for the current turn to finish.",
  },
};

export const WorktreeBindingForkAvailable: Story = {
  args: { state: "worktree", label: "payments-retry", onFork: () => {} },
};

/**
 * UNWIRED — presentational only.
 *
 * A persisted session cannot be rebound and there is no draft-session state, so
 * nothing in the composer can reach this. It ships so the chip's vocabulary is
 * complete and so the materialization flow has a rendering target the day the
 * daemon offers one. See the Task 07 report, blocker B1.
 */
export const Presentational_New: Story = {
  args: { state: "new", label: "new: docs-refresh", presentational: true },
};

/** UNWIRED — presentational only (see `Presentational_New`). */
export const Presentational_Pending: Story = {
  args: { state: "pending", label: "docs-refresh", mono: "bootstrap", presentational: true },
};

/** UNWIRED — presentational only (see `Presentational_New`). */
export const Presentational_Failed: Story = {
  args: { state: "failed", label: "docs-refresh", presentational: true },
};

export const AllStates: Story = {
  args: RootBinding.args,
  render: () => (
    <StorySurface className="flex flex-wrap items-center gap-3 p-6">
      <SessionEnvironmentChip label="compozy" mono="root" state="root" />
      <SessionEnvironmentChip
        forkUnavailableReason="Start or resume the session before forking it."
        label="payments-retry"
        state="worktree"
      />
      <SessionEnvironmentChip label="payments-retry" onFork={() => {}} state="worktree" />
      <SessionEnvironmentChip label="new: docs-refresh" presentational state="new" />
      <SessionEnvironmentChip label="docs-refresh" presentational state="pending" />
      <SessionEnvironmentChip label="docs-refresh" presentational state="failed" />
    </StorySurface>
  ),
};
