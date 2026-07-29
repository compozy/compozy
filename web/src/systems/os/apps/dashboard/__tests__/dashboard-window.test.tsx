import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TopbarSlotProvider } from "@compozy/ui";

const connection = { status: "connected" as "connected" | "disconnected" };
const desktop = {
  activeDesktopId: "desktop-1",
  focusedId: "win-1",
  windows: {
    "win-1": { desktopId: "desktop-1", minimized: false },
  },
};

vi.mock("../../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: typeof desktop) => unknown) => selector(desktop),
}));

vi.mock("@/systems/status", () => ({
  useDaemonHealth: () => ({
    connectionStatus: connection.status,
    health: undefined,
    isInitialLoading: false,
  }),
}));

vi.mock("@/systems/session", () => ({
  useSessionCreateActions: () => ({ openForAgent: vi.fn() }),
  useSessionCreateHasActiveWorkspace: () => true,
  useSessionCreateIsCreating: () => false,
}));

// Spy on the body so this suite proves the shell's connection gate (body mounts
// only while connected; disconnect surface replaces it otherwise) instead of the
// stub's markup — testing-boss R22: test the behavior, never the mock.
const homeDashboardSpy = vi.fn((_props: { liveEnabled: boolean }) => null);

vi.mock("@/systems/dashboard", () => ({
  HomeDashboard: (props: { liveEnabled: boolean }) => homeDashboardSpy(props),
}));

import { DashboardWindow } from "../dashboard-window";

function renderWindow() {
  return render(
    <TopbarSlotProvider>
      <DashboardWindow windowId="win-1" />
    </TopbarSlotProvider>
  );
}

describe("DashboardWindow", () => {
  beforeEach(() => {
    homeDashboardSpy.mockClear();
    desktop.activeDesktopId = "desktop-1";
    desktop.focusedId = "win-1";
    desktop.windows["win-1"] = { desktopId: "desktop-1", minimized: false };
  });

  it("Should mount the home dashboard body while connected", () => {
    connection.status = "connected";
    renderWindow();
    expect(homeDashboardSpy).toHaveBeenCalledWith({ liveEnabled: true });
    expect(screen.queryByTestId("home-error")).toBeNull();
  });

  it.each([
    {
      name: "another window has focus",
      arrange: () => {
        desktop.focusedId = "another-window";
      },
    },
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
  ])("Should keep Home live reads dormant when $name", ({ arrange }) => {
    connection.status = "connected";
    arrange();

    renderWindow();

    expect(homeDashboardSpy).toHaveBeenCalledWith({ liveEnabled: false });
  });

  it("Should render the disconnect surface when the daemon is unreachable", () => {
    connection.status = "disconnected";
    renderWindow();
    expect(screen.getByTestId("home-error")).toBeInTheDocument();
    expect(homeDashboardSpy).not.toHaveBeenCalled();
  });
});
