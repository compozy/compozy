import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import {
  LONG_WORKTREE_PATH,
  buildWorktreeFixture,
  worktreeListingFixture,
} from "@/systems/workspace/mocks";

import type {
  WorktreeCreateDialogModel,
  WorktreeCreateDraft,
} from "../../hooks/use-worktree-create-dialog";
import { buildWorktreeCreatePreview, deriveWorktreeParentDir } from "../../lib/worktree-naming";
import type { WorktreeRefusal } from "../../lib/worktree-refusal";
import { WorktreeCreateDialog } from "../worktree-create-dialog";

const GENERATED_NAME = "steady-heron";
const PARENT_DIR = deriveWorktreeParentDir(worktreeListingFixture.worktrees);

/**
 * Presentational harness: the dialog is driven by an explicit model, so each
 * visual-contract state is reachable without racing a real mutation.
 */
function useHarnessModel(overrides: Partial<WorktreeCreateDialogModel>): WorktreeCreateDialogModel {
  const [draft, setDraftState] = useState<WorktreeCreateDraft>({
    name: "",
    branch: "",
    baseRef: "",
    existingBranch: "",
    ...overrides.draft,
  });
  const [advancedOpen, setAdvancedOpen] = useState(overrides.advancedOpen ?? false);
  const effectiveName = draft.name.trim() === "" ? GENERATED_NAME : draft.name;

  return {
    draft,
    setDraft: next => setDraftState(current => ({ ...current, ...next })),
    advancedOpen,
    setAdvancedOpen,
    generatedName: GENERATED_NAME,
    preview: buildWorktreeCreatePreview(effectiveName, draft.branch, PARENT_DIR),
    branchCandidates: worktreeListingFixture.worktrees.map(worktree => ({
      branch: worktree.branch,
      heldBy: worktree.name,
    })),
    refusal: null,
    fieldError: null,
    heldByWorktree: null,
    isSubmitting: false,
    canSubmit: true,
    submit: fn(),
    reset: fn(),
    pendingWorktree: null,
    cancelCreate: fn(),
    isCancelling: false,
    cancelError: null,
    creationError: null,
    ...overrides,
  };
}

function Harness(overrides: Partial<WorktreeCreateDialogModel>) {
  const model = useHarnessModel(overrides);
  return (
    <StorySurface>
      <WorktreeCreateDialog
        open
        onOpenChange={fn()}
        workspaceName="launch-hq"
        model={model}
        onSelectHoldingWorktree={fn()}
      />
    </StorySurface>
  );
}

function refusal(overrides: Partial<WorktreeRefusal>): WorktreeRefusal {
  return {
    code: "worktree_name_taken",
    message: "A worktree named docs-refresh already exists.",
    risk: null,
    downgrade: false,
    details: null,
    ...overrides,
  };
}

const meta: Meta<typeof WorktreeCreateDialog> = {
  title: "systems/workspace/components/WorktreeCreateDialog",
  component: WorktreeCreateDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Name-first worktree creation. The generated name rides the placeholder, the branch → path preview derives live, and every refusal lands on its own field.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-22 — simple: one field plus the generated-name placeholder. */
export const Simple: Story = {
  args: {},
  render: () => <Harness preview={{ branch: GENERATED_NAME, path: LONG_WORKTREE_PATH }} />,
};

/**
 * VC-23 — advanced expanded: branch, base ref, and the held-branch picker.
 * Play opens the real existing-branch CommandSelect so free and held groups
 * are both capturable.
 */
export const AdvancedExpanded: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByTestId("worktree-create-existing-branch"));
  },
  render: () => (
    <Harness
      advancedOpen
      draft={{
        name: "docs-refresh",
        branch: "pedro/docs-refresh",
        baseRef: "main",
        existingBranch: "",
      }}
      branchCandidates={[
        { branch: "main", heldBy: null },
        { branch: "pedro/docs-refresh", heldBy: null },
        { branch: "pedro/payments-retry", heldBy: "payments-retry" },
      ]}
    />
  ),
};

/** VC-24 — name collision refused on the name field. */
export const NameCollision: Story = {
  args: {},
  render: () => (
    <Harness
      draft={{ name: "docs-refresh", branch: "", baseRef: "", existingBranch: "" }}
      refusal={refusal({})}
      fieldError="name"
      canSubmit={false}
    />
  ),
};

/** VC-25 — the branch is held elsewhere; the escape is selecting that worktree. */
export const BranchHeldElsewhere: Story = {
  args: {},
  render: () => (
    <Harness
      advancedOpen
      draft={{
        name: "auth-refresh-2",
        branch: "",
        baseRef: "",
        existingBranch: "pedro/auth-refresh",
      }}
      refusal={refusal({
        code: "branch_held_by_worktree",
        message: "Branch pedro/auth-refresh is checked out in auth-refresh.",
        details: { worktree: "auth-refresh" },
      })}
      fieldError="branch"
      canSubmit={false}
      heldByWorktree={buildWorktreeFixture({ name: "auth-refresh" })}
    />
  ),
};

/** VC-26 — base ref not found, refused on the base-ref field. */
export const BaseRefMissing: Story = {
  args: {},
  render: () => (
    <Harness
      advancedOpen
      draft={{ name: "docs-refresh", branch: "", baseRef: "trunk", existingBranch: "" }}
      refusal={refusal({
        code: "base_ref_not_found",
        message: "Base ref trunk was not found.",
      })}
      fieldError="baseRef"
      canSubmit={false}
    />
  ),
};

/**
 * VC-27 — submitting. Cancel stays live because the daemon already accepted the
 * create; once the pending row exists, cancelling reaches it by id.
 */
export const SubmittingWithPendingRow: Story = {
  args: {},
  render: () => (
    <Harness
      draft={{ name: "docs-refresh", branch: "", baseRef: "", existingBranch: "" }}
      isSubmitting
      canSubmit={false}
      pendingWorktree={buildWorktreeFixture({
        name: "docs-refresh",
        state: "pending",
        pending_phase: "branch",
      })}
    />
  ),
};
