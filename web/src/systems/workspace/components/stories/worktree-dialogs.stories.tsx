import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import {
  LONG_WORKTREE_PATH,
  buildWorktreeFixture,
  discoveredWorktreeFixture,
  worktreeMissingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import type { WorktreeRefusal } from "../../lib/worktree-refusal";
import { WorktreeAdoptDialog } from "../worktree-adopt-dialog";
import { WorktreeMissingResolutionDialog } from "../worktree-missing-resolution-dialog";
import { WorktreeRemoveDialog } from "../worktree-remove-dialog";

const DIRTY_RISK = {
  changedFiles: 6,
  insertions: 142,
  deletions: 18,
  unpushedCommits: 3,
  existsOnRemote: false,
};

function refusal(overrides: Partial<WorktreeRefusal>): WorktreeRefusal {
  return {
    code: "worktree_dirty_requires_force",
    message: "The worktree holds work that exists nowhere else.",
    risk: DIRTY_RISK,
    downgrade: false,
    details: null,
    ...overrides,
  };
}

const cleanWorktree = buildWorktreeFixture({
  name: "fix-flaky-e2e",
  branch: "run/fix-flaky-e2e",
  path: LONG_WORKTREE_PATH,
});

const meta: Meta<typeof WorktreeAdoptDialog> = {
  title: "systems/workspace/components/WorktreeLifecycleDialogs",
  component: WorktreeAdoptDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Adoption, removal, and missing-resolution dialogs. Every destructive transition is explicit, quantified, and refusable (ADR-006).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-13 — adoption confirm naming the validation. */
export const AdoptConfirm: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeAdoptDialog
        open
        onOpenChange={fn()}
        discovered={discoveredWorktreeFixture}
        refusal={null}
        isAdopting={false}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-14 — adoption refused because the metadata resolves into the main checkout. */
export const AdoptRefusal: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeAdoptDialog
        open
        onOpenChange={fn()}
        discovered={discoveredWorktreeFixture}
        refusal={refusal({
          code: "adoption_main_checkout",
          message:
            "Its metadata resolves into the main checkout, not a linked worktree. Nothing was changed.",
          risk: null,
        })}
        isAdopting={false}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-28 — clean removal: the branch is explicitly not deleted. */
export const RemoveClean: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={cleanWorktree}
        refusal={null}
        forceRequested={false}
        onRequestForce={fn()}
        isRemoving={false}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-29 — a bound idle session will be stopped, and its history survives. */
export const RemoveBoundIdle: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={cleanWorktree}
        refusal={null}
        forceRequested={false}
        onRequestForce={fn()}
        boundSessions={{ running: 0, idle: 1 }}
        isRemoving={false}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-30 — a session mid-turn blocks removal outright. */
export const RemoveBoundRunning: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={cleanWorktree}
        refusal={refusal({ code: "worktree_session_active", risk: null })}
        forceRequested={false}
        onRequestForce={fn()}
        boundSessions={{ running: 1, idle: 0 }}
        isRemoving={false}
        onConfirm={fn()}
        onOpenSession={fn()}
      />
    </StorySurface>
  ),
};

/** VC-31 — dirty refusal with quantified risk rows and the exit flow as primary. */
export const RemoveDirtyRefusal: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={worktreeReadyDirtyRunningFixture}
        refusal={refusal({})}
        forceRequested={false}
        onRequestForce={fn()}
        isRemoving={false}
        onConfirm={fn()}
        onCommitAndPush={fn()}
      />
    </StorySurface>
  ),
};

/** VC-32 — force confirm re-states the exact quantities before destroying them. */
export const RemoveForce: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={worktreeReadyDirtyRunningFixture}
        refusal={refusal({})}
        forceRequested
        onRequestForce={fn()}
        isRemoving={false}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-33 — the branch also exists on a remote, so the refusal downgrades. */
export const RemoveRemoteDowngrade: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeRemoveDialog
        open
        onOpenChange={fn()}
        worktree={buildWorktreeFixture({ name: "data-migration" })}
        refusal={refusal({
          downgrade: true,
          risk: { ...DIRTY_RISK, changedFiles: 0, unpushedCommits: 1, existsOnRemote: true },
        })}
        forceRequested={false}
        onRequestForce={fn()}
        isRemoving={false}
        onConfirm={fn()}
      />
    </StorySurface>
  ),
};

/** VC-34 — missing resolution, choice view: dismiss the record, or say it's back. */
export const MissingResolution: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeMissingResolutionDialog
        open
        onOpenChange={fn()}
        worktree={worktreeMissingFixture}
        outcome={null}
        refusal={null}
        isPending={false}
        onDismissRecord={fn()}
        onRestore={fn()}
      />
    </StorySurface>
  ),
};

/** VC-34 — missing resolution, outcome view: the idempotent no-op, rendered verbatim. */
export const MissingResolutionIdempotentOutcome: Story = {
  args: {},
  render: () => (
    <StorySurface>
      <WorktreeMissingResolutionDialog
        open
        onOpenChange={fn()}
        worktree={worktreeMissingFixture}
        outcome="Nothing to clean up — no branch, worktree directory, or instance data found."
        refusal={null}
        isPending={false}
        onDismissRecord={fn()}
        onRestore={fn()}
      />
    </StorySurface>
  ),
};
