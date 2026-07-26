// Suite: Bundle activation detail
// Invariant: Detail states and lifecycle controls always reflect the daemon activation payload.
// Boundary IN: Bundle activation query plus update/deactivate mutation hooks.
// Boundary OUT: Generated transport and cache invalidation, owned by adapter/hook suites.
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";
import { useTopbarSlotValue, type TopbarSlotValue } from "@agh/ui";
import { bundleActivationFixtures } from "../../mocks/fixtures";
import type { BundleActivation } from "../../types";

const mocks = vi.hoisted(() => ({
  deactivate: {
    error: null as Error | null,
    isPending: false,
    mutateAsync: vi.fn(),
  },
  navigate: vi.fn(),
  query: {
    data: undefined as BundleActivation | undefined,
    error: null as Error | null,
    isLoading: false,
  },
  update: {
    isPending: false,
    mutate: vi.fn(),
  },
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...props }: { children?: ReactNode; to: string }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
  useNavigate: () => mocks.navigate,
}));

vi.mock("../../hooks/use-extensions", () => ({
  useBundleActivation: () => mocks.query,
}));

vi.mock("../../hooks/use-extension-actions", () => ({
  useDeactivateBundle: () => mocks.deactivate,
  useUpdateBundleActivation: () => mocks.update,
}));

import { BundleActivationDetail } from "../bundle-activation-detail";

function TopbarSlotProbe({ onSlot }: { onSlot: (slot: TopbarSlotValue | null) => void }) {
  const slot = useTopbarSlotValue();
  useEffect(() => {
    onSlot(slot);
  }, [onSlot, slot]);
  return null;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.query.data = bundleActivationFixtures[0];
  mocks.query.error = null;
  mocks.query.isLoading = false;
  mocks.update.isPending = false;
  mocks.deactivate.error = null;
  mocks.deactivate.isPending = false;
  mocks.deactivate.mutateAsync.mockResolvedValue(undefined);
});

describe("BundleActivationDetail", () => {
  it("Should replace stale detail with explicit loading, failure, and not-found states", () => {
    mocks.query.data = undefined;
    mocks.query.isLoading = true;
    const { rerender } = renderWithTopbar(<BundleActivationDetail id="activation-missing" />);
    expect(screen.queryByTestId("bundle-activation-detail")).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Loading bundle activation");

    mocks.query.isLoading = false;
    mocks.query.error = new Error("daemon unavailable");
    rerender(<BundleActivationDetail id="activation-missing" />);
    expect(screen.getByText("Unable to load bundle activation")).toBeInTheDocument();
    expect(screen.getByText("daemon unavailable")).toBeInTheDocument();

    mocks.query.error = null;
    rerender(<BundleActivationDetail id="activation-missing" />);
    expect(screen.getByText("Bundle activation not found")).toBeInTheDocument();
  });

  it("Should render daemon inventory and explicitly confirm a Live requirement on update", async () => {
    const user = userEvent.setup();
    mocks.query.data = { ...bundleActivationFixtures[0]!, spec_drift: true };
    let slotValue: TopbarSlotValue | null = null;
    const getSlot = () => slotValue;
    renderWithTopbar(
      <>
        <BundleActivationDetail id="activation-ops-starter" />
        <TopbarSlotProbe onSlot={slot => (slotValue = slot)} />
      </>
    );

    expect(screen.getAllByText("incident-commander").length).toBeGreaterThan(0);
    expect(screen.getAllByText("daily-status").length).toBeGreaterThan(0);
    expect(screen.getByText("session-failure-alert")).toBeInTheDocument();
    expect(screen.getAllByText("ops-slack").length).toBeGreaterThan(0);
    expect(screen.getByText("incidents")).toBeInTheDocument();
    expect(screen.getByText("Update available")).toBeInTheDocument();
    expect(screen.getByText("Live confirmed")).toBeInTheDocument();
    expect(screen.queryByText("Bind primary channel as default")).not.toBeInTheDocument();
    expect(getSlot()?.crumb).toBe("ops-starter");

    await user.click(screen.getByRole("switch", { name: "Confirm Live network participation" }));
    await user.click(screen.getByRole("button", { name: "Update" }));
    expect(mocks.update.mutate).toHaveBeenCalledWith({
      body: { confirm_network_requirement: true, expected_version: 7 },
      id: "activation-ops-starter",
    });
  });

  it("Should make a digest-only Network confirmation actionable", async () => {
    const user = userEvent.setup();
    mocks.query.data = {
      ...bundleActivationFixtures[0]!,
      network_requirement_confirmed_at: undefined,
      network_requirement_confirmed_by: undefined,
      spec_drift: false,
    };

    renderWithTopbar(<BundleActivationDetail id="activation-ops-starter" />);

    expect(screen.getByRole("button", { name: "Update" })).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: "Confirm Live network participation" })
    ).toBeInTheDocument();
    await user.click(screen.getByRole("switch", { name: "Confirm Live network participation" }));
    await user.click(screen.getByRole("button", { name: "Update" }));
    expect(mocks.update.mutate).toHaveBeenCalledWith({
      body: { confirm_network_requirement: true, expected_version: 7 },
      id: "activation-ops-starter",
    });
  });

  it("Should deactivate through the real dialog and return to the Bundles tab", async () => {
    const user = userEvent.setup();
    renderWithTopbar(<BundleActivationDetail id="activation-ops-starter" />);

    await user.click(screen.getByRole("button", { name: "Actions for ops-starter" }));
    await user.click(await screen.findByRole("menuitem", { name: "Deactivate…" }));
    await user.click(screen.getByRole("button", { name: "Deactivate" }));

    await waitFor(() =>
      expect(mocks.deactivate.mutateAsync).toHaveBeenCalledWith("activation-ops-starter")
    );
    expect(mocks.navigate).toHaveBeenCalledWith({
      search: { tab: "installed" },
      to: "/marketplace/bundles",
    });
  });

  it("Should show honest fallbacks when an activation has no optional inventory", () => {
    mocks.query.data = {
      ...bundleActivationFixtures[1]!,
      bundle_description: undefined,
      inventory: undefined,
      profile_description: undefined,
    };
    renderWithTopbar(<BundleActivationDetail id="activation-dep-kit" />);

    expect(screen.getByText("No bundle description is available.")).toBeInTheDocument();
    expect(screen.getByText("No profile description is available.")).toBeInTheDocument();
    expect(screen.getByText("No inventory entries.")).toBeInTheDocument();
    expect(screen.getAllByText("None in this activation.")).toHaveLength(5);
  });

  it("Should render scope and semantic timestamps", () => {
    renderWithTopbar(<BundleActivationDetail id="activation-ops-starter" />);

    expect(screen.getAllByText("workspace").length).toBeGreaterThan(0);
    expect(screen.getByText(/ws_northstar/i)).toBeInTheDocument();

    const timestamps = screen.getByText("Timestamps").closest("section");
    expect(timestamps).not.toBeNull();
    const times = within(timestamps as HTMLElement).getAllByRole("time");
    expect(times).toHaveLength(2);
    expect(times[0]).toHaveAttribute("dateTime", "2026-07-08T09:20:00Z");
    expect(times[1]).toHaveAttribute("dateTime", "2026-07-12T16:40:00Z");
  });
});
