// Suite: worktree checkout path display
// Invariant: a worktree filesystem path stays in the document as visible text
// and exposes the same full string in a tooltip, so compact dialogs and nest
// rows can truncate without hiding the destination.
// Owning layer: workspace domain components. Canonical suite: this file —
// submenu suites assert names and order, not path-tooltip behavior.
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import { WorktreePath } from "../worktree-path";

const PATH = "/Users/ada/.compozy/worktrees/launch-hq/payments-retry";

function renderPath(path = PATH) {
  return render(
    <TooltipProvider delay={0}>
      <WorktreePath path={path} />
    </TooltipProvider>
  );
}

describe("WorktreePath", () => {
  it("Should render the checkout path as visible text", () => {
    renderPath();

    expect(screen.getByText(PATH)).toBeVisible();
  });

  it("Should expose the full path in a tooltip", async () => {
    const user = userEvent.setup();
    renderPath();

    await user.hover(screen.getByText(PATH));

    await waitFor(() => {
      expect(
        screen.getByText(PATH, { selector: "[data-slot=tooltip-content]" })
      ).toBeInTheDocument();
    });
  });

  it("Should expose the full path when reached by keyboard", async () => {
    const user = userEvent.setup();
    const { container } = renderPath();

    await user.tab();

    expect(container.firstElementChild).toHaveFocus();
    await waitFor(() => {
      expect(
        screen.getByText(PATH, { selector: "[data-slot=tooltip-content]" })
      ).toBeInTheDocument();
    });
  });
});
