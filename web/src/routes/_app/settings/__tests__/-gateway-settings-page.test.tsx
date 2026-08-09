// Suite: Gateway settings route composition
// Invariant: the page owns loading/error composition and exposes pairing controls only on listeners
// that support pairing.
// Owning layer: the Gateway settings route page.
import { fireEvent, render, screen } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
  tier: "private" as "private" | "public" | undefined,
  view: {} as Record<string, unknown>,
}));

vi.mock("@/systems/gateway", () => ({
  GatewayAuditPanel: () => <div data-testid="gateway-audit-panel" />,
  GatewayDeviceList: ({ onPair }: { onPair?: () => void }) => (
    <div data-testid="gateway-device-list">
      {onPair ? (
        <button data-testid="gateway-pair-device" onClick={onPair} type="button">
          Pair a device
        </button>
      ) : null}
    </div>
  ),
  GatewayExposureSection: () => <div data-testid="gateway-exposure-section" />,
  GatewayPairingDialog: () => <div data-testid="gateway-pairing-dialog" />,
  GatewayProviderSection: () => <div data-testid="gateway-provider-section" />,
  useGatewayAccessTier: () => state.tier,
  useGatewaySettingsPage: () => state.view,
}));

vi.mock("@/systems/settings", () => ({
  SettingsPageFrame: ({ children }: PropsWithChildren) => <main>{children}</main>,
  useSettingsTopbar: vi.fn(),
}));

import { GatewaySettingsPage } from "../-gateway-settings-page";

function gatewayView() {
  return {
    page: {
      isLoading: false,
      error: null as Error | null,
      exposure: { localOnly: true },
      status: { addresses: [] },
      activeDevices: [],
      devices: [],
      refetch: vi.fn(),
    },
    exposure: { error: null, isSubmitting: false, setSurface: vi.fn() },
    providers: {
      actionError: null,
      inventoryError: null,
      isLoading: false,
      isSubmitting: false,
      model: { candidates: [], orphaned: [] },
      disable: vi.fn(),
      enable: vi.fn(),
    },
    devices: { error: null, isBusy: false, rename: vi.fn(), revoke: vi.fn() },
    audit: { error: null, hasRun: false, isRunning: false, report: null, run: vi.fn() },
    pairing: {
      address: "https://private.gateway.test",
      artifact: null,
      error: null,
      isMinting: false,
      mint: vi.fn(),
      open: false,
      setOpen: vi.fn(),
      start: vi.fn(),
    },
  };
}

describe("GatewaySettingsPage", () => {
  beforeEach(() => {
    state.tier = "private";
    state.view = gatewayView();
  });

  it("Should render private pairing controls with the supported gateway sections", () => {
    render(<GatewaySettingsPage />);

    expect(screen.getByTestId("gateway-pair-device")).toBeInTheDocument();
    expect(screen.getByTestId("gateway-pairing-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("gateway-audit-panel")).toBeInTheDocument();
  });

  it("Should hide pairing while retaining safe management on the public operator tier", () => {
    state.tier = "public";
    render(<GatewaySettingsPage />);

    expect(screen.queryByTestId("gateway-pair-device")).not.toBeInTheDocument();
    expect(screen.queryByTestId("gateway-pairing-dialog")).not.toBeInTheDocument();
    expect(screen.getByTestId("gateway-exposure-section")).toBeInTheDocument();
    expect(screen.getByTestId("gateway-device-list")).toBeInTheDocument();
  });

  it("Should own the loading state", () => {
    state.view = { ...gatewayView(), page: { ...gatewayView().page, isLoading: true } };
    render(<GatewaySettingsPage />);

    expect(screen.getByTestId("settings-page-gateway-loading")).toBeInTheDocument();
  });

  it("Should retry a failed page load", () => {
    const view = gatewayView();
    view.page.error = new Error("Gateway unavailable");
    state.view = view;
    render(<GatewaySettingsPage />);

    expect(screen.getByTestId("settings-page-gateway-error")).toHaveTextContent(
      "Gateway unavailable"
    );
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(view.page.refetch).toHaveBeenCalledTimes(1);
  });
});
