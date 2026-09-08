// Suite: Settings window nav
// Invariant: a plain click on a section link is window navigation handed to the shell (the router
// ignores a navigation to the current location), while modified clicks keep native link behavior.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { describe, expect, it, vi } from "vitest";

import { SettingsWindowNav } from "../settings-window-nav";

async function renderNav(initialPath: string) {
  const onNavigate = vi.fn();
  const rootRoute = createRootRoute({
    component: () => (
      <SettingsWindowNav activeSlug="general" connection="connected" onNavigate={onNavigate} />
    ),
  });
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  });
  render(<RouterProvider router={router} />);
  await screen.findByRole("link", { name: "Profiles" });
  return { onNavigate, router };
}

describe("SettingsWindowNav", () => {
  it("Should hand a plain click to the shell even when the location already reads the section", async () => {
    const user = userEvent.setup();
    const { onNavigate, router } = await renderNav("/settings/profiles");

    await user.click(screen.getByRole("link", { name: "Profiles" }));

    expect(onNavigate).toHaveBeenCalledOnce();
    expect(onNavigate).toHaveBeenCalledWith({ pathname: "/settings/profiles", search: {} });
    expect(router.state.location.pathname).toBe("/settings/profiles");
  });

  it("Should leave a modified click to the native link", async () => {
    const user = userEvent.setup();
    const { onNavigate } = await renderNav("/settings/general");
    const nativeNavigation = vi.fn((event: MouseEvent) => {
      const prevented = event.defaultPrevented;
      // Observe the browser handoff after React, then stop the native I/O
      // boundary: jsdom cannot navigate another document.
      event.preventDefault();
      return prevented;
    });
    document.addEventListener("click", nativeNavigation, { once: true });

    try {
      await user.keyboard("{Meta>}");
      await user.click(screen.getByRole("link", { name: "Profiles" }));
      await user.keyboard("{/Meta}");
    } finally {
      document.removeEventListener("click", nativeNavigation);
    }

    expect(onNavigate).not.toHaveBeenCalled();
    expect(nativeNavigation).toHaveBeenCalledOnce();
    expect(nativeNavigation).toHaveReturnedWith(false);
    expect(screen.getByRole("link", { name: "Profiles" })).toHaveAttribute(
      "href",
      "/settings/profiles"
    );
  });
});
