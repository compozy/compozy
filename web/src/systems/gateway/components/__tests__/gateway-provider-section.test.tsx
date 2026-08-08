// Suite: connectivity provider surface
// Invariant: each provider's name and live health come from the gateway status payload, the empty
// state says nothing is installed rather than implying otherwise, a blocked provider cannot be
// enabled, and enabling sends the real install source, digest and generation fence.
// Owning layer: the gateway provider components.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { buildGatewayProviderModel } from "../../lib/gateway-provider-model";
import {
  connectivityProviderFixture,
  gatewayStatusFixture,
  gatewayTierFixture,
} from "../../mocks/fixtures";
import { GatewayProviderSection } from "../gateway-provider-section";

function renderSection(
  model: ReturnType<typeof buildGatewayProviderModel>,
  overrides: Partial<React.ComponentProps<typeof GatewayProviderSection>> = {}
) {
  const props = {
    model,
    isLoading: false,
    isSubmitting: false,
    onEnable: vi.fn(),
    onDisable: vi.fn(),
    ...overrides,
  };
  render(<GatewayProviderSection {...props} />);
  return props;
}

describe("GatewayProviderSection", () => {
  it("Should say no provider is installed and name the next action", () => {
    renderSection(buildGatewayProviderModel(gatewayStatusFixture(), []));

    expect(screen.getByText("No connectivity provider installed")).toBeInTheDocument();
    expect(screen.getByTestId("gateway-provider-empty")).toHaveTextContent("Settings → Extensions");
  });

  it("Should not claim the inventory is empty while it is still loading", () => {
    renderSection(buildGatewayProviderModel(gatewayStatusFixture(), []), { isLoading: true });

    expect(screen.getByRole("status", { name: "Loading connectivity providers" })).toBeVisible();
    expect(screen.queryByText("No connectivity provider installed")).not.toBeInTheDocument();
  });

  it("Should report an inventory failure without claiming that nothing is installed", () => {
    renderSection(buildGatewayProviderModel(gatewayStatusFixture(), []), {
      inventoryError: "extension inventory unavailable",
    });

    expect(screen.getByRole("alert")).toHaveTextContent("extension inventory unavailable");
    expect(screen.queryByText("No connectivity provider installed")).not.toBeInTheDocument();
  });

  it("Should show each provider's name and live health from gateway status", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({
        tier: "private",
        observed: "degraded",
        verified: false,
        cause: "provider handshake failed",
      })
    );
    renderSection(
      buildGatewayProviderModel(status, [
        connectivityProviderFixture({ name: "connectivity-fixture" }),
      ])
    );

    const row = screen.getByTestId("gateway-provider-connectivity-fixture");
    expect(row).toHaveTextContent("connectivity-fixture");
    expect(
      screen.getByTestId("gateway-provider-health-connectivity-fixture-private")
    ).toHaveTextContent("Degraded");
    expect(
      screen.getByTestId("gateway-provider-cause-connectivity-fixture-private")
    ).toHaveTextContent("provider handshake failed");
  });

  it("Should refuse to enable a provider whose extension is not ready", () => {
    const { onEnable } = renderSection(
      buildGatewayProviderModel(gatewayStatusFixture(), [
        connectivityProviderFixture({ enabled: false }),
      ])
    );
    const enable = screen.getByTestId("gateway-provider-connectivity-overlay-private-enable");

    expect(
      screen.getByTestId("gateway-provider-connectivity-overlay-blocker-extension-disabled")
    ).toHaveTextContent("Settings → Extensions");
    expect(enable).toBeDisabled();
    fireEvent.click(enable);
    expect(onEnable).not.toHaveBeenCalled();
  });

  it("Should enable a ready provider with the source and digest the daemon fences on", () => {
    const onEnable = vi.fn();
    renderSection(
      buildGatewayProviderModel(gatewayStatusFixture(), [connectivityProviderFixture()]),
      { onEnable }
    );
    fireEvent.click(screen.getByTestId("gateway-provider-connectivity-overlay-private-enable"));

    expect(onEnable).toHaveBeenCalledTimes(1);
    expect(onEnable).toHaveBeenCalledWith(
      expect.objectContaining({
        digest: "sha256:overlay-control-digest",
        installSource: "bundled",
        name: "connectivity-overlay",
      }),
      "private"
    );
  });

  it("Should offer disable for the tier a provider already serves", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "up", verified: true })
    );
    const { onDisable } = renderSection(
      buildGatewayProviderModel(status, [
        connectivityProviderFixture({ name: "connectivity-fixture" }),
      ])
    );
    fireEvent.click(screen.getByTestId("gateway-provider-connectivity-fixture-private-disable"));

    expect(onDisable).toHaveBeenCalledWith("connectivity-fixture", "private");
  });

  it("Should render a daemon refusal verbatim instead of implying the change applied", () => {
    renderSection(buildGatewayProviderModel(gatewayStatusFixture(), []), {
      actionError: "gateway digest confirmation required",
    });

    expect(screen.getByTestId("gateway-provider-error")).toHaveTextContent(
      "gateway digest confirmation required"
    );
  });

  it("Should keep an activation whose extension was removed disableable", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "up", verified: true })
    );
    const { onDisable } = renderSection(buildGatewayProviderModel(status, []));
    fireEvent.click(
      screen.getByTestId("gateway-provider-orphan-connectivity-fixture-private-disable")
    );

    expect(onDisable).toHaveBeenCalledWith("connectivity-fixture", "private");
  });

  it("Should render unknown provider tokens with a neutral fallback", () => {
    const status = gatewayStatusFixture({
      providers: [
        {
          cause: "",
          desired: "enabled",
          generation: 1,
          health: "future-health",
          name: "removed-provider",
          observed: "future-state",
          tier: "future-tier",
        },
      ],
    });
    renderSection(buildGatewayProviderModel(status, []));

    const row = screen.getByTestId("gateway-provider-orphan-removed-provider-unknown");
    expect(row).toHaveTextContent("Unknown");
    expect(row).toHaveTextContent("State unavailable");
    expect(row).toHaveTextContent("unknown address");
    expect(row).not.toHaveTextContent("future-health");
    expect(row).not.toHaveTextContent("future-state");
    expect(
      screen.getByTestId("gateway-provider-orphan-removed-provider-unknown-disable")
    ).toBeDisabled();
  });
});
