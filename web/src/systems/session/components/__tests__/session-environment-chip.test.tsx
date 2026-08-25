import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import { SessionEnvironmentChip } from "../session-environment-chip";

/**
 * UT-133 — the composer environment control.
 *
 * Invariant: the live environment control is icon-only, exposes its target and
 * fork action through its accessible name and tooltip, and only invokes the
 * fork handler when the daemon reports it available. Owning layer: this
 * component.
 */
describe("SessionEnvironmentChip", () => {
  const renderChip = (ui: ReactNode) => render(<TooltipProvider delay={0}>{ui}</TooltipProvider>);

  it("Should state an unbound workspace as an icon-only, focusable fork control", () => {
    const workspacePath = "/Users/pedronauck/Dev/compozy";
    renderChip(<SessionEnvironmentChip label={workspacePath} state="root" />);

    const button = screen.getByRole("button", {
      name: `Workspace: ${workspacePath} — fork into a new worktree`,
    });
    expect(button).toHaveAttribute("data-binding", "root");
    expect(button).toHaveAttribute("data-locked", "");
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).not.toHaveAttribute("title");
    expect(button.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    expect(screen.queryByText(workspacePath)).not.toBeInTheDocument();
  });

  it("Should describe the current workspace and fork action on focus", async () => {
    const user = userEvent.setup();
    const workspacePath = "/Users/pedronauck/Dev/compozy";
    renderChip(<SessionEnvironmentChip label={workspacePath} state="root" />);

    await user.tab();

    await waitFor(() => {
      expect(
        screen.getByText(`Workspace: ${workspacePath} — fork into a new worktree`)
      ).toBeInTheDocument();
    });
  });

  it("Should surface the daemon's unavailable reason verbatim", async () => {
    const user = userEvent.setup();
    const reason = "Wait for the current turn to finish.";
    renderChip(
      <SessionEnvironmentChip
        forkUnavailableReason={reason}
        label="payments-retry"
        state="worktree"
      />
    );

    const button = screen.getByRole("button", {
      name: /Worktree: payments-retry — fork into a new worktree/,
    });
    expect(button).toHaveAttribute("data-binding", "worktree");
    expect(button).toHaveAttribute("data-fork", "unavailable");
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).not.toHaveAttribute("title");

    await user.hover(button);

    await waitFor(() => {
      expect(
        screen.getByText(`Worktree: payments-retry — fork into a new worktree. ${reason}`)
      ).toBeInTheDocument();
    });
  });

  it("Should offer the fork only when the daemon reports it available", async () => {
    const user = userEvent.setup();
    const onFork = vi.fn();
    renderChip(<SessionEnvironmentChip label="payments-retry" onFork={onFork} state="worktree" />);

    const button = screen.getByRole("button", {
      name: "Worktree: payments-retry — fork into a new worktree",
    });
    expect(button).toHaveAttribute("data-fork", "available");
    expect(button).not.toHaveAttribute("aria-disabled");
    await user.click(button);
    expect(onFork).toHaveBeenCalledTimes(1);
  });

  it("Should never render an environment picker in any state", () => {
    renderChip(<SessionEnvironmentChip label="payments-retry" onFork={vi.fn()} state="worktree" />);

    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  /**
   * These three states have no live wiring: a persisted session cannot be
   * rebound, so nothing in the composer can reach them. They are marked so a
   * reviewer can tell a presentational state from shipped behaviour.
   */
  it.each(["new", "pending", "failed"] as const)(
    "Should render the unwired %s state as an icon-only presentational control",
    state => {
      renderChip(<SessionEnvironmentChip label="docs-refresh" presentational state={state} />);

      const button = screen.getByRole("button", { name: "Workspace: docs-refresh" });
      expect(button).toHaveAttribute("data-presentational", "true");
      expect(button).toHaveAttribute("data-state", state);
      expect(button).toHaveAttribute("aria-disabled", "true");
      expect(button.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
      expect(screen.queryByText("docs-refresh")).not.toBeInTheDocument();
    }
  );
});
