import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from "@tanstack/react-router";
import { describe, expect, it } from "vitest";

import { useRouterScrollTracking } from "@/hooks/use-router-scroll-tracking";

import { restoreScrollOutsideShell } from "../router-scroll-restoration";

// Invariant (#1546): the router keeps scroll bookkeeping only for the pages outside the shell,
// the only ones it restores. Shell routes leave no recorded offsets; `/design-system` gets its
// offset back after navigating away and back. Owning layer: the router's scroll policy wiring
// in `main.tsx`, exercised on a real router with the same policy.
function renderRouter(initialPath: string) {
  const rootRoute = createRootRoute({ component: () => <Outlet /> });
  const shellRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div data-testid="shell-scroller" />,
  });
  const otherShellRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/sessions",
    component: () => <div data-testid="shell-other" />,
  });
  const designSystemRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/design-system",
    component: function DesignSystemPage() {
      useRouterScrollTracking();
      return <div data-testid="design-system-scroller" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([shellRoute, otherShellRoute, designSystemRoute]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  });
  restoreScrollOutsideShell(router);
  render(<RouterProvider router={router} />);
  return router;
}

describe("restoreScrollOutsideShell", () => {
  it("Should record no scroll offsets for shell routes", async () => {
    const router = renderRouter("/");
    const scroller = await screen.findByTestId("shell-scroller");
    scroller.scrollTop = 120;
    fireEvent.scroll(scroller);

    await act(() => router.navigate({ to: "/sessions" }));
    await screen.findByTestId("shell-other");
    fireEvent(window, new Event("pagehide"));

    expect(window.sessionStorage.length).toBe(0);
  });

  it("Should restore the design-system page's offset after navigating away and back", async () => {
    const router = renderRouter("/design-system");
    const scroller = await screen.findByTestId("design-system-scroller");
    scroller.scrollTop = 200;
    fireEvent.scroll(scroller);

    await act(() => router.navigate({ to: "/" }));
    await screen.findByTestId("shell-scroller");
    act(() => router.history.back());

    const restored = await screen.findByTestId("design-system-scroller");
    await waitFor(() => expect(restored.scrollTop).toBe(200));
  });
});
