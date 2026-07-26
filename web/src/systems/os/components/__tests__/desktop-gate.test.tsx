// Suite: desktop onboarding gate
// Invariant: first-run setup renders over the live shell, and completing it reveals the
// desktop that was already mounted behind the panel rather than building a new one.
// Boundary IN: DesktopGate's status branching and child lifecycle.
// Boundary OUT: the setup panel internals, the onboarding transport, and the window manager.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  status: {
    data: undefined as { completed: boolean } | undefined,
    isError: false,
    error: null as unknown,
    refetch: vi.fn(),
  },
}));

vi.mock("@/systems/onboarding", () => ({
  useOnboardingStatus: () => mocks.status,
  OnboardingSetupPanel: ({ onComplete }: { onComplete: () => void }) => (
    <div data-testid="onboarding-setup-panel">
      <button type="button" onClick={onComplete}>
        Finish setup
      </button>
    </div>
  ),
}));

import { DesktopGate } from "../desktop-gate";

let chromeMounts = 0;

function Chrome() {
  useEffect(() => {
    chromeMounts += 1;
  }, []);
  return <div data-testid="os-desktop">desktop</div>;
}

function renderGate() {
  return render(
    <DesktopGate>
      <Chrome />
    </DesktopGate>
  );
}

describe("DesktopGate", () => {
  beforeEach(() => {
    chromeMounts = 0;
    mocks.status.data = undefined;
    mocks.status.isError = false;
    mocks.status.error = null;
    mocks.status.refetch = vi.fn();
  });

  it("Should render the shell behind the setup panel while setup is incomplete", () => {
    mocks.status.data = { completed: false };

    renderGate();

    expect(screen.getByTestId("os-desktop")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-setup-panel")).toBeInTheDocument();
    expect(screen.queryByTestId("onboarding-gate-loading")).toBeNull();
  });

  it("Should reveal the desktop on completion without remounting it", async () => {
    const user = userEvent.setup();
    mocks.status.data = { completed: false };
    const { rerender } = renderGate();
    expect(chromeMounts).toBe(1);

    await user.click(screen.getByRole("button", { name: "Finish setup" }));
    expect(mocks.status.refetch).toHaveBeenCalledOnce();

    mocks.status.data = { completed: true };
    rerender(
      <DesktopGate>
        <Chrome />
      </DesktopGate>
    );

    expect(screen.getByTestId("os-desktop")).toBeInTheDocument();
    expect(screen.queryByTestId("onboarding-setup-panel")).toBeNull();
    expect(chromeMounts).toBe(1);
  });

  it("Should render neither the shell nor the panel before the daemon answers", () => {
    renderGate();

    expect(screen.getByTestId("onboarding-gate-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("os-desktop")).toBeNull();
    expect(screen.queryByTestId("onboarding-setup-panel")).toBeNull();
  });

  it("Should offer a retry when the status read fails, with no shell behind it", async () => {
    const user = userEvent.setup();
    mocks.status.isError = true;
    mocks.status.error = new Error("daemon unreachable");

    renderGate();

    expect(screen.getByTestId("onboarding-gate-error")).toHaveTextContent("daemon unreachable");
    expect(screen.queryByTestId("os-desktop")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.status.refetch).toHaveBeenCalledOnce();
  });
});
