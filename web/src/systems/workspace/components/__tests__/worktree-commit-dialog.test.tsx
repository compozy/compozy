import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { WorktreeExitLadderRow } from "../../lib/worktree-exit-ladder";
import type { WorktreeExitCommitScope } from "../../types";
import { AGENT_MESSAGE_PROMPT } from "../worktree-commit-dialog-copy";
import { WorktreeCommitDialog } from "../worktree-commit-dialog";

/**
 * Invariant: the agent affordance stages a reviewable prompt and never runs
 * git; with no session bound it becomes the honest alternative rather than a
 * dead control. Owning layer: this dialog.
 */
const SCOPE: WorktreeExitCommitScope = {
  changed_files: 6,
  insertions: 142,
  deletions: 18,
  untracked_files: ["docs/notes.md"],
  untracked_total: 1,
};

const COMMIT_ACTION: WorktreeExitLadderRow = {
  action: "commit",
  label: "Commit",
  enabled: true,
  publish: false,
  isPrimary: true,
  requiresInput: true,
  opensURL: false,
};

function renderDialog(overrides: Partial<React.ComponentProps<typeof WorktreeCommitDialog>> = {}) {
  const onCommit = overrides.onCommit ?? vi.fn();
  render(
    <WorktreeCommitDialog
      actions={[COMMIT_ACTION]}
      onCommit={onCommit}
      onOpenChange={vi.fn()}
      open
      scope={SCOPE}
      worktreeName="payments-retry"
      {...overrides}
    />
  );
  return { onCommit };
}

describe("WorktreeCommitDialog entity shell", () => {
  it("Should mount as an unframed ruled shell so chrome padding is not stacked", () => {
    renderDialog();

    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("data-frame", "unframed");
    expect(dialog.querySelector('[data-slot="dialog-header"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
    expect(dialog.querySelector('[data-slot="dialog-footer"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
  });
});

describe("WorktreeCommitDialog agent affordance", () => {
  it("Should stage a reviewable prompt without committing when a session is bound", async () => {
    const user = userEvent.setup();
    const onStagePrompt = vi.fn();
    const { onCommit } = renderDialog({ onStagePrompt });

    await user.click(screen.getByRole("button", { name: "Have the agent write it" }));

    expect(onStagePrompt).toHaveBeenCalledWith(AGENT_MESSAGE_PROMPT);
    expect(onCommit).not.toHaveBeenCalled();
  });

  it("Should tell the agent not to run git in the staged prompt", () => {
    expect(AGENT_MESSAGE_PROMPT).toMatch(/do not run git/i);
  });

  it("Should confirm the prompt landed in the composer once staged", () => {
    renderDialog({ onStagePrompt: vi.fn(), promptStaged: true });

    expect(screen.getByText("Prompt staged in the session composer.")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Have the agent write it" })
    ).not.toBeInTheDocument();
  });

  it("Should offer to start a session when none is bound, without committing", async () => {
    const user = userEvent.setup();
    const onStartSession = vi.fn();
    const { onCommit } = renderDialog({ onStartSession });

    const start = screen.getByRole("button", { name: "Start a session in this worktree" });
    expect(
      screen.queryByRole("button", { name: "Have the agent write it" })
    ).not.toBeInTheDocument();

    await user.click(start);
    expect(onStartSession).toHaveBeenCalledTimes(1);
    expect(onCommit).not.toHaveBeenCalled();
  });

  it("Should render no agent affordance when neither path is available", () => {
    renderDialog();

    expect(
      screen.queryByRole("button", { name: "Have the agent write it" })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Start a session in this worktree" })
    ).not.toBeInTheDocument();
  });
});
