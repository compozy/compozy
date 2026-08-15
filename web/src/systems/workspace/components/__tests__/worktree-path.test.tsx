// Suite: worktree checkout path display
// Invariant: a worktree filesystem path stays in the document as visible text
// and exposes the full absolute string in a tooltip while home-rooted visible
// paths contract to `~`, so compact dialogs and nest rows stay readable.
// Owning layer: workspace domain components. Canonical suite: this file —
// submenu suites assert names and order, not path-tooltip behavior.
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import { WorktreePath } from "../worktree-path";

const PATH = "/Users/ada/.compozy/worktrees/launch-hq/payments-retry";

function renderPath(path = PATH, userHomeDir?: string) {
  return render(
    <TooltipProvider delay={0}>
      <WorktreePath path={path} userHomeDir={userHomeDir} />
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

  it.each([
    { name: "exact home", path: "/Users/ada", home: "/Users/ada", want: "~" },
    {
      name: "home child",
      path: "/Users/ada/dev/compozy",
      home: "/Users/ada",
      want: "~/dev/compozy",
    },
    { name: "trailing home separator", path: "/Users/ada/dev", home: "/Users/ada/", want: "~/dev" },
    {
      name: "sibling prefix",
      path: "/Users/adam/dev",
      home: "/Users/ada",
      want: "/Users/adam/dev",
    },
    { name: "missing home", path: "/repo/compozy", home: undefined, want: "/repo/compozy" },
  ])(
    "Should display $name without changing the absolute tooltip path",
    async ({ path, home, want }) => {
      const user = userEvent.setup();
      const { container } = renderPath(path, home);
      const visiblePath = container.querySelector<HTMLElement>('[data-slot="worktree-path"]');
      if (!visiblePath) throw new Error("worktree path trigger did not render");

      expect(visiblePath).toHaveTextContent(want);
      expect(visiblePath).toBeVisible();
      await user.hover(visiblePath);
      await waitFor(() => {
        expect(
          screen.getByText(path, { selector: "[data-slot=tooltip-content]" })
        ).toBeInTheDocument();
      });
    }
  );
});
