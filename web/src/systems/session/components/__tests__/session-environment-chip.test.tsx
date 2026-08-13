import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SessionEnvironmentChip } from "../session-environment-chip";

/**
 * UT-133 — the composer environment chip.
 *
 * Invariant: a live session's binding is immutable, so the chip states the
 * binding and never offers an in-place switch; fork availability and its reason
 * come from the daemon's command catalog. Owning layer: this component.
 */
describe("SessionEnvironmentChip", () => {
  it("Should state a workspace-root binding without a lock or a fork affordance", () => {
    render(<SessionEnvironmentChip label="compozy" mono="root" state="root" />);

    const chip = screen.getByText("compozy").closest("[data-slot='session-environment-chip']");
    expect(chip).toHaveAttribute("data-binding", "root");
    expect(chip).toHaveAttribute("data-locked");
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("Should render a worktree binding as inert when the daemon refuses the fork", () => {
    const reason = "Wait for the current turn to finish.";
    render(
      <SessionEnvironmentChip
        forkUnavailableReason={reason}
        label="payments-retry"
        state="worktree"
      />
    );

    const chip = screen
      .getByText("payments-retry")
      .closest("[data-slot='session-environment-chip']");
    expect(chip).toHaveAttribute("data-fork", "unavailable");
    expect(chip).toHaveAttribute("title", reason);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("Should offer the fork only when the daemon reports it available", async () => {
    const user = userEvent.setup();
    const onFork = vi.fn();
    render(<SessionEnvironmentChip label="payments-retry" onFork={onFork} state="worktree" />);

    const chip = screen.getByRole("button", { name: /fork to move/i });
    expect(chip).toHaveAttribute("data-fork", "available");
    await user.click(chip);
    expect(onFork).toHaveBeenCalledTimes(1);
  });

  it("Should never render an environment picker in any live state", () => {
    render(<SessionEnvironmentChip label="payments-retry" onFork={vi.fn()} state="worktree" />);

    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  /**
   * These three states have no live wiring: a persisted session cannot be
   * rebound, so nothing in the composer can reach them. They are marked so a
   * reviewer can tell a presentational state from a shipped behaviour.
   */
  it.each(["new", "pending", "failed"] as const)(
    "Should mark the unwired %s state as presentational",
    state => {
      render(<SessionEnvironmentChip label="docs-refresh" presentational state={state} />);

      const chip = screen
        .getByText("docs-refresh")
        .closest("[data-slot='session-environment-chip']");
      expect(chip).toHaveAttribute("data-presentational", "true");
      expect(chip).toHaveAttribute("data-state", state);
    }
  );
});
