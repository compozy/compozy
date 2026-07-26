// Suite: OS dock state
// Invariant: every launcher exposes closed, running, or minimized from its real item state;
// OpenDesign proximity magnification scales the nearest icon and stays off when gated.
// Boundary IN: OsDock item semantics + pointer proximity.
// Boundary OUT: WM-to-dock projection (DesktopDock) and browser lifecycle journeys.
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Bot, LayoutDashboard, ListChecks } from "lucide-react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@agh/ui";

import { OsDock } from "../os-dock";

function renderDock(ui: React.ReactElement) {
  return render(<TooltipProvider delay={0}>{ui}</TooltipProvider>);
}

describe("OsDock", () => {
  beforeEach(() => {
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      cb(0);
      return 1;
    });
    vi.stubGlobal("cancelAnimationFrame", () => undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should expose the real closed, running, and minimized state for each launcher", () => {
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard, running: true },
          { id: "tasks", name: "Tasks", icon: ListChecks, minimized: true },
          { id: "agents", name: "Agents", icon: Bot },
        ]}
        onSelect={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "Dashboard" })).toHaveAttribute(
      "data-state",
      "running"
    );
    expect(screen.getByRole("button", { name: "Tasks" })).toHaveAttribute(
      "data-state",
      "minimized"
    );
    expect(screen.getByRole("button", { name: "Agents" })).toHaveAttribute("data-state", "closed");
  });

  it("Should hide zero badges and cap large attention counts at 9+ (UT-066)", () => {
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard, badge: 0 },
          { id: "tasks", name: "Tasks", icon: ListChecks, badge: 12 },
        ]}
        onSelect={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "Dashboard" })).not.toHaveTextContent("0");
    expect(screen.getByRole("button", { name: "Tasks" })).toHaveTextContent("9+");
  });

  it("Should show the launcher name in a tooltip on focus", async () => {
    const user = userEvent.setup();
    renderDock(
      <OsDock
        magnify={false}
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard },
          { id: "network", name: "Network", icon: Bot },
        ]}
        onSelect={vi.fn()}
      />
    );

    await user.tab();
    await user.tab();
    expect(screen.getByRole("button", { name: "Network" })).toHaveFocus();
    await waitFor(() => {
      expect(screen.getByText("Network")).toBeInTheDocument();
    });
  });

  it("Should magnify the nearest launcher on pointer proximity", () => {
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard },
          { id: "tasks", name: "Tasks", icon: ListChecks },
        ]}
        onSelect={vi.fn()}
      />
    );

    const dock = screen.getByRole("navigation", { name: "Dock" });
    const dashboard = screen.getByRole("button", { name: "Dashboard" });
    vi.spyOn(dashboard, "getBoundingClientRect").mockReturnValue({
      left: 100,
      width: 46,
      top: 0,
      height: 46,
      right: 146,
      bottom: 46,
      x: 100,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerMove(dock, { clientX: 123 });

    expect(dashboard.style.transform).toContain("scale(");
    expect(dashboard.style.transform).toContain("translateY(");
  });

  it("Should keep launchers static when magnification is disabled", () => {
    renderDock(
      <OsDock
        magnify={false}
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard },
          { id: "tasks", name: "Tasks", icon: ListChecks },
        ]}
        onSelect={vi.fn()}
      />
    );

    const dock = screen.getByRole("navigation", { name: "Dock" });
    const dashboard = screen.getByRole("button", { name: "Dashboard" });
    vi.spyOn(dashboard, "getBoundingClientRect").mockReturnValue({
      left: 100,
      width: 46,
      top: 0,
      height: 46,
      right: 146,
      bottom: 46,
      x: 100,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerMove(dock, { clientX: 123 });

    expect(dashboard.style.transform).toBe("");
  });
});
