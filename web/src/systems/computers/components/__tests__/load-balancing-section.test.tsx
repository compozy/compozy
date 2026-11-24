// Suite: Settings → Computers Load balancing section & host load projection (US-015, U1)
// Invariant: Load balancing stays hidden with fewer than 2 connected computers when off, shows
//            a collapsed summary, toggles load balancing, updates this-computer and per-computer
//            weights, omits this computer when local execution is off, and marks stale/null
//            resource samples as Unsampled.
// Boundary IN: LoadBalancingSection and toHostLoadView.
// Boundary OUT: Daemon HTTP mutations (covered in use-local-computer-settings.test.tsx).

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import { toHostLoadView } from "../../lib/placement-view";
import {
  COMPUTERS_FREE,
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
} from "../../mocks/remote-computers-fixtures";
import { LoadBalancingSection } from "../load-balancing-section";

function renderSection(props: Parameters<typeof LoadBalancingSection>[0]) {
  return render(
    <TooltipProvider>
      <LoadBalancingSection {...props} />
    </TooltipProvider>
  );
}

describe("LoadBalancingSection (US-015)", () => {
  it("hides when fewer than 2 connected computers exist and load balancing is off", () => {
    const { container } = renderSection({
      local: LOCAL_COMPUTER,
      computers: [],
      enabled: false,
      onToggleEnabled: vi.fn(),
      onChangeLocalWeight: vi.fn(),
      onChangeComputerWeight: vi.fn(),
    });
    expect(container).toBeEmptyDOMElement();

    const { container: controllerOnly } = renderSection({
      local: { ...LOCAL_COMPUTER, localRunAgents: false },
      computers: COMPUTERS_FREE,
      enabled: false,
      onToggleEnabled: vi.fn(),
      onChangeLocalWeight: vi.fn(),
      onChangeComputerWeight: vi.fn(),
    });
    expect(controllerOnly).toBeEmptyDOMElement();
  });

  it("shows collapsed summary and allows toggling load balancing and weights for local + remote computers", () => {
    const onToggleEnabled = vi.fn();
    const onChangeLocalWeight = vi.fn();
    const onChangeComputerWeight = vi.fn();

    renderSection({
      local: LOCAL_COMPUTER,
      computers: COMPUTERS_PRO,
      enabled: true,
      defaultOpen: true,
      onToggleEnabled,
      onChangeLocalWeight,
      onChangeComputerWeight,
    });

    expect(screen.getByTestId("computers-load-balancing")).toBeInTheDocument();
    expect(screen.getByTestId("computers-load-balancing-summary")).toHaveTextContent(
      "studio prefer"
    );

    fireEvent.click(screen.getByRole("switch", { name: "Enable load balancing" }));
    expect(onToggleEnabled).toHaveBeenCalledWith(false);

    fireEvent.change(screen.getByTestId("load-balancing-weight-local"), {
      target: { value: "100" },
    });
    expect(onChangeLocalWeight).toHaveBeenCalledWith(100);

    fireEvent.change(screen.getByTestId("load-balancing-weight-cmp_7f3k2q9x"), {
      target: { value: "0" },
    });
    expect(onChangeComputerWeight).toHaveBeenCalledWith("cmp_7f3k2q9x", 0);
  });

  it("omits this computer weight row when local execution is off and disables selects while off", () => {
    renderSection({
      local: { ...LOCAL_COMPUTER, localRunAgents: false },
      computers: COMPUTERS_PRO,
      enabled: false,
      defaultOpen: true,
      onToggleEnabled: vi.fn(),
      onChangeLocalWeight: vi.fn(),
      onChangeComputerWeight: vi.fn(),
    });

    expect(screen.getByTestId("computers-load-balancing-summary")).toHaveTextContent("Off");
    expect(screen.queryByTestId("load-balancing-weight-local")).not.toBeInTheDocument();
    expect(screen.getByTestId("load-balancing-weight-cmp_1c9d0a2b")).toBeDisabled();
  });

  it("projects fresh host resource samples and marks stale or incomplete samples as unsampled", () => {
    const now = Date.parse("2026-11-24T12:00:10Z");
    const fresh = toHostLoadView(
      {
        cpu_count: 8,
        cpu_util: 0.25,
        mem_total: 16_000_000_000,
        mem_available: 8_000_000_000,
        sampled_at: "2026-11-24T12:00:05Z",
      },
      now
    );
    expect(fresh).toMatchObject({
      cpuCount: 8,
      cpuUtil: 0.25,
      stale: false,
      sampledAgoLabel: "5s ago",
    });

    const stale = toHostLoadView(
      {
        cpu_count: 8,
        cpu_util: 0.25,
        mem_total: 16_000_000_000,
        mem_available: 8_000_000_000,
        sampled_at: "2026-11-24T11:59:50Z",
      },
      now
    );
    expect(stale?.stale).toBe(true);

    expect(
      toHostLoadView(
        {
          cpu_count: 8,
          cpu_util: null,
          mem_total: 16_000_000_000,
          mem_available: 8_000_000_000,
          sampled_at: "2026-11-24T12:00:05Z",
        },
        now
      )
    ).toBeNull();
  });
});
