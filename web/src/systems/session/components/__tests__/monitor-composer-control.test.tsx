import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { SessionMonitorPayload } from "../../adapters/session-monitor-api";
import { MonitorComposerControl } from "../monitor-composer-control";

// Suite: composer monitor control.
// Invariant: the control appears only while the session has active monitors; its popover says
// what each monitor watches and when it fires or gives up, and cancels a waiting monitor.
// Owning layer: session presentational components (props in, callbacks out). Canonical suite: this file.
const FUTURE = new Date(Date.now() + 12 * 60_000).toISOString();

function monitor(overrides: Partial<SessionMonitorPayload> = {}): SessionMonitorPayload {
  return {
    monitor_id: "mon-1",
    session_id: "sess-1",
    kind: "timer",
    description: "Check CI on PR #875",
    status: "active",
    created_at: new Date().toISOString(),
    deadline_at: FUTURE,
    ...overrides,
  };
}

function renderControl(props: Partial<Parameters<typeof MonitorComposerControl>[0]> = {}) {
  return render(
    <UIProvider>
      <MonitorComposerControl active={1} monitors={[]} {...props} />
    </UIProvider>
  );
}

describe("MonitorComposerControl", () => {
  it("Should render nothing while the session has no active monitor", () => {
    renderControl({ active: 0 });
    expect(screen.queryByTestId("composer-monitors-button")).not.toBeInTheDocument();
  });

  it("Should list what each monitor watches and cancel a waiting one", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    const onOpenChange = vi.fn();
    renderControl({
      active: 2,
      onCancel,
      onOpenChange,
      monitors: [
        monitor(),
        monitor({
          monitor_id: "mon-2",
          kind: "terminal",
          description: "Wait for the release build",
          until: "match",
          pattern: "published v\\d+",
        }),
      ],
    });

    const trigger = screen.getByRole("button", { name: "Monitors, 2 active" });
    expect(trigger).toHaveTextContent("2");
    await user.click(trigger);
    expect(onOpenChange).toHaveBeenLastCalledWith(true);

    const timer = screen.getByTestId("composer-monitor-mon-1");
    expect(timer).toHaveTextContent("Check CI on PR #875");
    expect(timer).toHaveTextContent(/Timer·fires in 1[12]m/);
    const terminal = screen.getByTestId("composer-monitor-mon-2");
    expect(terminal).toHaveTextContent("Terminal output matches published v\\d+");
    expect(terminal).toHaveTextContent(/gives up in 1[12]m/);

    await user.click(
      within(timer).getByRole("button", { name: "Cancel monitor: Check CI on PR #875" })
    );
    expect(onCancel).toHaveBeenCalledExactlyOnceWith("mon-1");
  });

  it("Should show a fired monitor as waking the session without a cancel action", async () => {
    const user = userEvent.setup();
    renderControl({ onCancel: vi.fn(), monitors: [monitor({ status: "delivering" })] });
    await user.click(screen.getByRole("button", { name: "Monitors, 1 active" }));

    const row = screen.getByTestId("composer-monitor-mon-1");
    expect(row).toHaveTextContent("Waking the session…");
    expect(within(row).queryByRole("button")).not.toBeInTheDocument();
  });

  // Invariant: a list that has not been read yet holds its place with one placeholder row per
  // active monitor the session reports (a few at most), named as one loading area, and never
  // claims that nothing is being monitored.
  it.each([
    { active: 1, rows: 1 },
    { active: 3, rows: 3 },
    { active: 9, rows: 4 },
  ])(
    "Should hold $rows placeholder rows for $active active monitors while loading",
    async ({ active, rows }) => {
      const user = userEvent.setup();
      renderControl({ active, loading: true });
      await user.click(screen.getByRole("button", { name: `Monitors, ${active} active` }));

      const region = screen.getByRole("status");
      expect(region).toHaveAttribute("aria-busy", "true");
      expect(region).toHaveTextContent("Loading monitors");
      expect(region.querySelectorAll("li")).toHaveLength(rows);
      expect(screen.queryByText("Nothing is being monitored.")).not.toBeInTheDocument();
    }
  );

  it("Should offer a retry when the list cannot be read", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderControl({ failed: true, onRetry });
    await user.click(screen.getByRole("button", { name: "Monitors, 1 active" }));

    expect(screen.getByText("Couldn't load monitors.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("Should say the session is gone without a retry when the list finds no session", async () => {
    const user = userEvent.setup();
    // Rows cached from an earlier read must not stay cancelable once the session is gone.
    renderControl({ failed: true, missing: true, onRetry: vi.fn(), monitors: [monitor()] });
    await user.click(screen.getByRole("button", { name: "Monitors, 1 active" }));

    expect(screen.getByText("This session is no longer available.")).toBeInTheDocument();
    expect(screen.queryByTestId("composer-monitor-mon-1")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
  });

  it("Should keep cached monitors visible and offer a retry when a refresh fails", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderControl({ failed: true, onRetry, monitors: [monitor()] });
    await user.click(screen.getByRole("button", { name: "Monitors, 1 active" }));

    expect(screen.getByTestId("composer-monitor-mon-1")).toHaveTextContent("Check CI on PR #875");
    expect(screen.getByText("Couldn't refresh monitors.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });
});
