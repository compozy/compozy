// Suite: worktree nest list rendering
// Invariant (UT-134, component half): rows render in the locked order, truncate at five behind an
// adopted-only overflow row, keyboard order matches visual order, discovered rows are selectable as
// the adoption gesture, and pending/missing/error rows are inert with their reason in a visible lane.
// Owning layer: workspace domain components. Canonical suite: this component test.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { toWorktreeNestEntries } from "../../lib/worktree-display";
import { adoptedWorktreeCount, sortWorktreeNestEntries } from "../../lib/worktree-sort";
import {
  discoveredWorktreeFixture,
  manyWorktreesListingFixture,
  worktreeErrorFixture,
  worktreeMissingFixture,
  worktreePendingFixture,
  worktreeReadyDirtyRunningFixture,
  worktreeListingFixture,
} from "../../mocks/worktree-fixtures";
import { WorktreeNestList } from "../worktree-nest-list";

/**
 * A nest small enough to survive truncation, so the inert and adoption rules
 * are asserted on rendered rows rather than on rows the limit cut away.
 */
const untruncatedListing = {
  worktrees: [
    worktreeReadyDirtyRunningFixture,
    worktreePendingFixture,
    worktreeMissingFixture,
    worktreeErrorFixture,
  ],
  discovered: [discoveredWorktreeFixture],
  repo: worktreeListingFixture.repo,
};

function renderNest(
  listing = worktreeListingFixture,
  overrides: Partial<React.ComponentProps<typeof WorktreeNestList>> = {}
) {
  const entries = sortWorktreeNestEntries(toWorktreeNestEntries(listing));
  const onSelectWorktree = vi.fn();
  render(
    <WorktreeNestList
      entries={entries}
      workspaceName="launch-hq"
      adoptedCount={adoptedWorktreeCount(entries)}
      onSelectWorktree={onSelectWorktree}
      {...overrides}
    />
  );
  return { entries, onSelectWorktree };
}

describe("WorktreeNestList", () => {
  it("Should label the nest by its parent workspace", () => {
    renderNest();

    expect(screen.getByRole("group", { name: "Worktrees in launch-hq" })).toBeInTheDocument();
  });

  it("Should render rows in the locked order", () => {
    const { entries } = renderNest();

    const rendered = screen
      .getAllByTestId(/worktree-nest-item-/)
      .map(node => node.textContent ?? "");
    // DOM order is the sorted order, which is also the keyboard order.
    entries.slice(0, 5).forEach((entry, index) => {
      expect(rendered[index]).toContain(entry.name);
    });
  });

  it("Should truncate at five and fold the rest behind an adopted-only overflow row", async () => {
    const user = userEvent.setup();
    const onShowAll = vi.fn();
    renderNest(manyWorktreesListingFixture, { onShowAll });

    expect(screen.getAllByTestId(/worktree-nest-item-/)).toHaveLength(5);
    // 9 adopted records + 1 discovered → the count says 9, never 10.
    const overflow = screen.getByTestId("worktree-nest-overflow");
    expect(overflow).toHaveTextContent(
      `All ${manyWorktreesListingFixture.worktrees.length} worktrees`
    );

    await user.click(overflow);
    expect(onShowAll).toHaveBeenCalled();
  });

  it("Should render no overflow row when the nest fits", () => {
    renderNest({
      ...worktreeListingFixture,
      worktrees: worktreeListingFixture.worktrees.slice(0, 2),
    });

    expect(screen.queryByTestId("worktree-nest-overflow")).toBeNull();
  });

  it("Should make a discovered row selectable as the adoption gesture", async () => {
    const user = userEvent.setup();
    const { onSelectWorktree } = renderNest(untruncatedListing);

    const discovered = screen.getByTestId(
      "worktree-nest-item-discovered:/Users/ada/dev/northstar-spike"
    );
    expect(within(discovered).getByText("Adopt")).toBeInTheDocument();

    await user.click(discovered);

    // Selecting it is the consent moment; it is never inert.
    expect(onSelectWorktree).toHaveBeenCalledWith(
      expect.objectContaining({ kind: "discovered", adoptable: true })
    );
  });

  it("Should keep pending, missing, and error rows inert with a visible reason", async () => {
    const user = userEvent.setup();
    const { onSelectWorktree } = renderNest(untruncatedListing);

    const pending = screen.getByTestId("worktree-nest-item-wt_docs_refresh");
    const missing = screen.getByTestId("worktree-nest-item-wt_hotfix_cors");
    const error = screen.getByTestId("worktree-nest-item-wt_bench_harness");

    expect(pending).toHaveAttribute("aria-disabled", "true");
    expect(missing).toHaveAttribute("aria-disabled", "true");
    expect(error).toHaveAttribute("aria-disabled", "true");
    // The reason sits in its own lane, not behind a hover.
    expect(within(pending).getByText("materializing")).toBeVisible();
    expect(within(missing).getByText("directory missing")).toBeVisible();
    expect(within(error).getByText("status read failed")).toBeVisible();

    await user.click(pending);
    await user.click(missing);
    await user.click(error);
    expect(onSelectWorktree).not.toHaveBeenCalled();
  });

  it("Should keep inert rows out of the keyboard tab order", () => {
    renderNest(untruncatedListing);

    const pending = screen.getByTestId("worktree-nest-item-wt_docs_refresh");
    const ready = screen.getByTestId("worktree-nest-item-wt_payments_retry");

    expect(pending).toHaveAttribute("tabindex", "-1");
    expect(ready).toHaveAttribute("tabindex", "0");
  });

  it("Should activate a row from the keyboard", async () => {
    const user = userEvent.setup();
    const { onSelectWorktree } = renderNest();

    screen.getByTestId("worktree-nest-item-wt_payments_retry").focus();
    await user.keyboard("{Enter}");

    expect(onSelectWorktree).toHaveBeenCalledWith(
      expect.objectContaining({ name: "payments-retry" })
    );
  });

  it("Should offer the create path when the workspace can host one", async () => {
    const user = userEvent.setup();
    const onCreateWorktree = vi.fn();
    renderNest(worktreeListingFixture, { onCreateWorktree });

    await user.click(screen.getByTestId("worktree-nest-create"));

    expect(onCreateWorktree).toHaveBeenCalled();
  });
});
