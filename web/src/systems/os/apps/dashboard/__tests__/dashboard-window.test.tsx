import { render, screen } from "@testing-library/react";
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

vi.mock("@/systems/status/hooks/use-daemon-health", () => ({
  useDaemonHealth: () => ({
    connectionStatus: connection.status,
    health: undefined,
    isInitialLoading: false,
  }),
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

function renderWindow() {
  return render(
    <TopbarSlotProvider>
      <Topbar title="Home" />
      <DashboardWindow windowId="win-1" />
    </TopbarSlotProvider>
  );
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

  it("Should mount the home dashboard body while connected", () => {
    connection.status = "connected";
    renderWindow();
    expect(homeDashboardSpy).toHaveBeenCalledWith({ liveEnabled: true });
    expect(screen.queryByTestId("home-error")).toBeNull();
  });

  it("Should disable New session in Global scope and say why", () => {
    connection.status = "connected";
    sessionCreate.hasActiveWorkspace = false;
    renderWindow();
    expect(screen.getByRole("button", { name: /New session/ })).toBeDisabled();
    expect(screen.getByTestId("home-new-session-disabled")).toHaveAccessibleName(
      "New session — pick a project to start a session"
    );
  });

  it("Should offer New session once a project is active", () => {
    connection.status = "connected";
    renderWindow();
    expect(screen.getByRole("button", { name: /New session/ })).toBeEnabled();
    expect(screen.queryByTestId("home-new-session-disabled")).toBeNull();
  });

  it("Should keep Home live while it remains visible without focus", () => {
    connection.status = "connected";
    desktop.focusedId = "another-window";

    renderWindow();

    expect(homeDashboardSpy).toHaveBeenCalledWith({ liveEnabled: true });
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
  ])("Should keep Home live reads dormant when $name", ({ arrange }) => {
    connection.status = "connected";
    arrange();

    renderWindow();

    expect(homeDashboardSpy).toHaveBeenCalledWith({ liveEnabled: false });
  });

  it("Should render the disconnect surface when the daemon is unreachable", () => {
    connection.status = "disconnected";
    renderWindow();
    expect(screen.getByTestId("home-error")).toHaveTextContent("CompozyOS isn't running");
    expect(homeDashboardSpy).not.toHaveBeenCalled();
  });
});
