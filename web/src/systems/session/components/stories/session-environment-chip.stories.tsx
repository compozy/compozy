import type { Meta, StoryObj } from "@storybook/react-vite";

import { storyWorkspacePaths } from "@/storybook/fintech-scenario";
import { StorySurface } from "@/storybook/story-layout";

import { SessionEnvironmentChip } from "../session-environment-chip";

const ROOT_LABEL = storyWorkspacePaths.hq;

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

/**
 * VC-07 — the workspace root, stated rather than offered.
 *
 * Production passes `session.workspace_path` as the label and never sets `mono`.
 */
export const RootBinding: Story = {
  args: { state: "root", label: ROOT_LABEL },
};

/** VC-08 — bound, with the daemon refusing the fork right now and saying why. */
export const WorktreeBindingForkUnavailable: Story = {
  args: {
    state: "worktree",
    label: "payments-retry",
    forkUnavailableReason: "Wait for the current turn to finish.",
  },
};

/** VC-12 — the binding is locked for a live session, so the chip leads to a fork. */
export const WorktreeBindingForkAvailable: Story = {
  args: { state: "worktree", label: "payments-retry", onFork: () => {} },
};

/**
 * VC-09 — the new-worktree target. Presentational only (D-07/D-08).
 *
 * A persisted session cannot be rebound, so the live composer never reaches
 * `new`, `pending`, or `failed`. Those states exist so the chip vocabulary is
 * complete. Creation phases and the three recovery exits are captured on
 * `SessionEnvironmentField` (VC-10, VC-11).
 */
export const Presentational_New: Story = {
  args: { state: "new", label: "new: docs-refresh", presentational: true },
};

/** Presentational only (D-07/D-08). See `Presentational_New`. */
export const Presentational_Pending: Story = {
  args: { state: "pending", label: "docs-refresh", presentational: true },
};

/** Presentational only (D-07/D-08). See `Presentational_New`. */
export const Presentational_Failed: Story = {
  args: { state: "failed", label: "docs-refresh", presentational: true },
};

export const AllStates: Story = {
  args: RootBinding.args,
  render: () => (
    <StorySurface className="flex flex-wrap items-center gap-3 p-6">
      <SessionEnvironmentChip label={ROOT_LABEL} state="root" />
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
