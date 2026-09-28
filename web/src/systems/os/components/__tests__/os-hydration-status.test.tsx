// Suite: desktop hydration status
// Invariant: the menubar carries the one window-layout status, in plain words,
// and stays silent when there is nothing to report.
// Boundary IN: OsHydrationStatus rendering and accessibility contract.
// Boundary OUT: WebSocket state transitions (hooks/__tests__/use-window-manager-stream.test.tsx).
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import { OsHydrationStatus } from "../os-hydration-status";

const STREAM_DIAGNOSTIC = {
  code: "stream_invalid",
  message: "The window-manager stream was invalid.",
  severity: "error",
  field: null,
} as const;

describe("OsHydrationStatus", () => {
  it("Should announce a plain recovery state while desktop sync is degraded", () => {
    render(
      <TooltipProvider>
        <OsHydrationStatus hydration="degraded" diagnostic={STREAM_DIAGNOSTIC} />
      </TooltipProvider>
    );

    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Can't save window layout — retrying");
    expect(status).not.toHaveTextContent(STREAM_DIAGNOSTIC.message);
  });

  it("Should report a reconnecting stream without warning tone", () => {
    render(<OsHydrationStatus hydration="live" connectionStatus="reconnecting" />);

    expect(screen.getByRole("status")).toHaveTextContent("Reconnecting…");
  });

  it("Should surface a refused command's plain notice while the stream is healthy", () => {
    render(
      <OsHydrationStatus
        hydration="live"
        connectionStatus="connected"
        diagnostic={{
          code: "window_manager_window_pinned",
          message: "Unpin the window first.",
          severity: "warning",
          field: null,
        }}
      />
    );

    expect(screen.getByRole("status")).toHaveTextContent("Unpin the window first.");
  });

  it.each(["pending", "live"] as const)(
    "Should not report a sync failure while hydration is %s",
    hydration => {
      render(<OsHydrationStatus hydration={hydration} />);

      expect(screen.queryByRole("status")).toBeNull();
    }
  );

  it("Should stay silent when no project binds a layout stream", () => {
    render(<OsHydrationStatus hydration="degraded" connectionStatus="disconnected" unbound />);

    expect(screen.queryByRole("status")).toBeNull();
  });
});
