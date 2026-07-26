import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TopbarSlotProvider } from "@agh/ui";

const connection = { status: "connected" as "connected" | "disconnected" };

vi.mock("@/systems/status", () => ({
  useDaemonHealth: () => ({
    connectionStatus: connection.status,
    health: undefined,
    isInitialLoading: false,
  }),
}));

vi.mock("@/systems/session", () => ({
  useSessionCreate: () => ({
    openForAgent: vi.fn(),
    isCreating: false,
    pendingAgentName: null,
    hasActiveWorkspace: true,
  }),
}));

// Spy on the body so this suite proves the shell's connection gate (body mounts
// only while connected; disconnect surface replaces it otherwise) instead of the
// stub's markup — testing-boss R22: test the behavior, never the mock.
const homeDashboardSpy = vi.fn(() => null);

vi.mock("@/systems/dashboard", () => ({
  HomeDashboard: () => homeDashboardSpy(),
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
  });

  it("Should mount the home dashboard body while connected", () => {
    connection.status = "connected";
    renderWindow();
    expect(homeDashboardSpy).toHaveBeenCalled();
    expect(screen.queryByTestId("home-error")).toBeNull();
  });

  it("Should render the disconnect surface when the daemon is unreachable", () => {
    connection.status = "disconnected";
    renderWindow();
    expect(screen.getByTestId("home-error")).toBeInTheDocument();
    expect(homeDashboardSpy).not.toHaveBeenCalled();
  });
});
