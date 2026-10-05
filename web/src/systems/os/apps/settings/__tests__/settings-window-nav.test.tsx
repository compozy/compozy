// Suite: Settings window nav
// Invariant: a plain click on a section link is window navigation handed to the shell (the router
// ignores a navigation to the current location), while modified clicks keep native link behavior.
// The active window's search shortcut also works from desktop focus, without taking a keystroke
// owned by a field, dialog, modified chord or another handler. Owner: Settings window navigation.
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { OsShellContext } from "../../../contexts/os-shell-context";
import { RoutingCoordinator } from "../../../lib/routing-coordinator";
import { windowManagerKeys } from "../../../lib/window-manager-query";
import {
  parseWindowManagerClientView,
  parseWindowManagerSnapshot,
} from "../../../lib/window-manager-schemas";
import { parseSettingsWindowManagerSection } from "../../../lib/window-manager-settings-section";
import {
  windowManagerClientFixture,
  windowManagerStorySnapshot,
  windowManagerStoryWindowId,
} from "../../../mocks/fixtures";
import { WindowManagerRuntime } from "../../../runtime/window-manager-runtime";
import { settingsWindowManagerSectionFixture } from "@/systems/settings/mocks/window-manager-fixtures";
import { statusOptions } from "@/systems/status/lib/query-options";
import { statusFixture } from "@/systems/status/mocks/fixtures";
import { SettingsWindow } from "../settings-window";
import { SettingsWindowNav } from "../settings-window-nav";

const cleanups: Array<() => void> = [];

afterEach(() => {
  for (const cleanup of cleanups.splice(0)) cleanup();
});

async function renderSettingsWindow() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const snapshot = parseWindowManagerSnapshot(windowManagerStorySnapshot("/settings/appearance"));
  const client = parseWindowManagerClientView(
    windowManagerClientFixture("client:settings-search", snapshot.workspaceId)
  );
  const snapshotKey = windowManagerKeys.snapshot(snapshot.workspaceId, "default");
  queryClient.setQueryData(snapshotKey, snapshot);
  queryClient.setQueryData(
    windowManagerKeys.config(snapshot.workspaceId, client.clientId),
    parseSettingsWindowManagerSection(settingsWindowManagerSectionFixture)
  );
  queryClient.setQueryData(statusOptions().queryKey, statusFixture);
  const manager = new WindowManagerRuntime(queryClient);
  manager.bind({
    workspaceId: snapshot.workspaceId,
    profileId: "default",
    clientId: client.clientId,
  });
  manager.start();
  manager.setClient(client);
  cleanups.push(() => {
    manager.destroy();
    queryClient.clear();
  });
  const shell = {
    projection: manager.projectionAtom,
    manager,
    coordinator: new RoutingCoordinator(manager, { navigate: () => {}, replace: () => {} }),
  };
  const rootRoute = createRootRoute({
    component: () => (
      <QueryClientProvider client={queryClient}>
        <OsShellContext value={shell}>
          <div data-testid="desktop" tabIndex={-1}>
            <SettingsWindow windowId={windowManagerStoryWindowId} />
            <div role="dialog" aria-label="Another action">
              <button type="button">Dialog action</button>
            </div>
            <button type="button" onKeyDown={event => event.preventDefault()}>
              Handles its own keys
            </button>
          </div>
        </OsShellContext>
      </QueryClientProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ["/settings/appearance"] }),
  });
  render(<RouterProvider router={router} />);
  const search = await screen.findByRole("searchbox", { name: "Search settings" });
  await screen.findByRole("radiogroup", { name: "Wallpaper" });
  return {
    search,
    desktop: screen.getByTestId("desktop"),
    manager,
    client,
    queryClient,
    snapshotKey,
    snapshot,
  };
}

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

describe("Settings window search shortcut", () => {
  it.each(["body", "desktop"])("Should focus search from %s focus", async target => {
    const user = userEvent.setup();
    const { search, desktop } = await renderSettingsWindow();
    if (target === "desktop") desktop.focus();
    else expect(document.body).toHaveFocus();

    await user.keyboard("/");

    expect(search).toHaveFocus();
    expect(search).toHaveValue("");
  });

  it("Should leave search typing, dialog actions and handled keys alone", async () => {
    const user = userEvent.setup();
    const { search } = await renderSettingsWindow();
    await user.type(search, "/");
    expect(search).toHaveFocus();
    expect(search).toHaveValue("/");

    for (const name of ["Dialog action", "Handles its own keys"]) {
      const button = screen.getByRole("button", { name });
      await user.click(button);
      await user.keyboard("/");
      expect(button).toHaveFocus();
    }
  });

  it.each(["metaKey", "ctrlKey", "altKey"])("Should preserve a %s chord", async modifier => {
    await renderSettingsWindow();
    const control = screen.getByRole("radio", { name: "Flat" });
    control.focus();

    expect(fireEvent.keyDown(control, { key: "/", [modifier]: true })).toBe(true);
    expect(control).toHaveFocus();
  });

  it("Should enable the shortcut only while its window is active", async () => {
    const user = userEvent.setup();
    const { desktop, search, manager, client, queryClient, snapshotKey, snapshot } =
      await renderSettingsWindow();
    desktop.focus();
    act(() =>
      manager.setClient({
        ...client,
        focusedWindowId: null,
        presentationRevision: client.presentationRevision + 1,
      })
    );
    await user.keyboard("/");
    expect(desktop).toHaveFocus();

    act(() =>
      manager.setClient({ ...client, presentationRevision: client.presentationRevision + 2 })
    );
    await user.keyboard("/");
    expect(search).toHaveFocus();

    desktop.focus();
    act(() => {
      queryClient.setQueryData(snapshotKey, {
        ...snapshot,
        windows: {
          ...snapshot.windows,
          [windowManagerStoryWindowId]: {
            ...snapshot.windows[windowManagerStoryWindowId],
            minimized: true,
          },
        },
      });
    });
    await user.keyboard("/");
    expect(desktop).toHaveFocus();
  });
});
