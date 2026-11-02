import { act, render, screen } from "@testing-library/react";
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Topbar, TopbarSlotProvider } from "@compozy/ui";

const connection = { status: "connected" as "connected" | "disconnected" };
const sessionCreate = { hasActiveWorkspace: true };
const desktop = {
  activeDesktopId: "desktop-1",
  focusedId: "win-1",
  windows: {
    "win-1": { desktopId: "desktop-1", minimized: false, stackActive: true },
  },
};

vi.mock("../../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: typeof desktop) => unknown) => selector(desktop),
}));

vi.mock("@/systems/status/hooks/use-daemon-connection-status", () => ({
  useDaemonConnectionStatus: () => connection.status,
}));

vi.mock("@/systems/session/hooks/use-session-create", () => ({
  useSessionCreateActions: () => ({ openForAgent: vi.fn() }),
  useSessionCreateHasActiveWorkspace: () => sessionCreate.hasActiveWorkspace,
  useSessionCreateIsCreating: () => false,
}));

// Spy on the body so this suite proves the shell's connection gate (body mounts
// only while connected; disconnect surface replaces it otherwise) instead of the
// stub's markup — testing-boss R22: test the behavior, never the mock.
const homeDashboardSpy = vi.fn((_props: { liveEnabled: boolean }) => null);

vi.mock("@/systems/dashboard/components/home-dashboard", () => ({
  HomeDashboard: (props: { liveEnabled: boolean }) => homeDashboardSpy(props),
}));

import { DashboardWindow } from "../dashboard-window";

async function renderWindow() {
  const rootRoute = createRootRoute({
    component: () => (
      <TopbarSlotProvider>
        <Topbar title="Home" />
        <DashboardWindow windowId="win-1" />
      </TopbarSlotProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  const result = render(<RouterProvider router={router} />);
  await act(() => router.load());
  return result;
}

describe("DashboardWindow", () => {
  beforeEach(() => {
    homeDashboardSpy.mockClear();
    sessionCreate.hasActiveWorkspace = true;
    desktop.activeDesktopId = "desktop-1";
    desktop.focusedId = "win-1";
    desktop.windows["win-1"] = {
      desktopId: "desktop-1",
      minimized: false,
      stackActive: true,
    };
  });

  it("Should mount the home dashboard body while connected", async () => {
    connection.status = "connected";
    await renderWindow();
    expect(homeDashboardSpy).toHaveBeenCalledWith(expect.objectContaining({ liveEnabled: true }));
    expect(screen.queryByTestId("home-error")).toBeNull();
  });

  it("Should rest New session while the destination settles", async () => {
    connection.status = "connected";
    sessionCreate.hasActiveWorkspace = false;
    await renderWindow();
    expect(screen.getByRole("button", { name: "New session" })).toBeDisabled();
  });

  it("Should offer New session once the destination has settled", async () => {
    connection.status = "connected";
    await renderWindow();
    expect(screen.getByRole("button", { name: "New session" })).toBeEnabled();
  });

  it("Should keep Home live while it remains visible without focus", async () => {
    connection.status = "connected";
    desktop.focusedId = "another-window";

    await renderWindow();

    expect(homeDashboardSpy).toHaveBeenCalledWith(expect.objectContaining({ liveEnabled: true }));
  });

  it.each([
    {
      name: "Home is minimized",
      arrange: () => {
        desktop.windows["win-1"].minimized = true;
      },
    },
    {
      name: "another desktop is active",
      arrange: () => {
        desktop.activeDesktopId = "desktop-2";
      },
    },
    {
      name: "another window in the stack is active",
      arrange: () => {
        desktop.windows["win-1"].stackActive = false;
      },
    },
  ])("Should keep Home live reads dormant when $name", async ({ arrange }) => {
    connection.status = "connected";
    arrange();

    await renderWindow();

    expect(homeDashboardSpy).toHaveBeenCalledWith(expect.objectContaining({ liveEnabled: false }));
  });

  it("Should render the disconnect surface when the daemon is unreachable", async () => {
    connection.status = "disconnected";
    await renderWindow();
    expect(screen.getByTestId("home-error")).toHaveTextContent("CompozyOS isn't running");
    expect(homeDashboardSpy).not.toHaveBeenCalled();
  });
});
