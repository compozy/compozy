import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { toWorktreeExitLadder } from "../../lib/worktree-exit-ladder";
import { exitPlanNoRemoteFixture } from "../../mocks/worktree-exit-fixtures";
import { WorktreeExitControl } from "../worktree-exit-control";

describe("WorktreeExitControl", () => {
  it("Should reveal daemon-computed alternatives without breaking the menu", async () => {
    const user = userEvent.setup();

    render(
      <WorktreeExitControl
        ladder={toWorktreeExitLadder(exitPlanNoRemoteFixture)}
        onAction={vi.fn()}
      />
    );

    await user.click(screen.getByRole("button", { name: "Git actions" }));

    const menu = await screen.findByRole("menu", { name: "Git actions" });

    expect(menu).toBeInTheDocument();
    expect(menu.querySelector("[data-slot='dropdown-menu-group']")).toBeInTheDocument();
  });
});
