// Suite: desktop hydration status
// Invariant: only degraded hydration reports a visible, accessible recovery state.
// Boundary IN: OsHydrationStatus rendering and accessibility contract.
// Boundary OUT: WebSocket state transitions (hooks/__tests__/use-window-manager-stream.test.tsx).
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { OsHydrationStatus } from "../os-hydration-status";

describe("OsHydrationStatus", () => {
  it("Should announce a non-blocking recovery state while desktop sync is degraded", () => {
    render(<OsHydrationStatus hydration="degraded" />);

    expect(
      screen.getByRole("status", {
        name: "Desktop sync paused. Changes stay in memory while AGH reconnects.",
      })
    ).toHaveTextContent("Sync paused · retrying");
  });

  it.each(["pending", "live"] as const)(
    "Should not report a sync failure while hydration is %s",
    hydration => {
      render(<OsHydrationStatus hydration={hydration} />);

      expect(screen.queryByRole("status")).toBeNull();
    }
  );
});
