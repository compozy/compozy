// Suite: gateway exposure orchestration
// Invariant: every public operator enable requires fresh consent, while disable remains immediate.
// Owning layer: the exposure section that coordinates row intent and the consent dialog.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { buildGatewayExposureModel } from "../../lib/gateway-exposure-model";
import { gatewayStatusFixture, gatewayTierFixture } from "../../mocks/fixtures";
import { GatewayExposureSection } from "../gateway-exposure-section";

function renderSection(exposure = buildGatewayExposureModel(gatewayStatusFixture())) {
  const onSetSurface = vi.fn();
  render(
    <GatewayExposureSection exposure={exposure} isSubmitting={false} onSetSurface={onSetSurface} />
  );
  return onSetSurface;
}

describe("GatewayExposureSection", () => {
  it("Should require consent before enabling public operator access", () => {
    const exposure = buildGatewayExposureModel(gatewayStatusFixture());
    const publicOperator = exposure.rows.find(row => row.id === "public-operator-ui");
    const onSetSurface = renderSection(exposure);

    fireEvent.click(screen.getByTestId("gateway-exposure-public-operator-ui-switch"));
    expect(screen.getByTestId("gateway-public-consent-dialog")).toBeInTheDocument();
    expect(onSetSurface).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("gateway-public-consent-acknowledge"));
    fireEvent.click(screen.getByTestId("gateway-public-consent-confirm"));
    expect(onSetSurface).toHaveBeenCalledWith(publicOperator, true, true);
  });

  it("Should disable public operator access without asking for consent", () => {
    const exposure = buildGatewayExposureModel(
      gatewayStatusFixture(
        gatewayTierFixture({
          tier: "public",
          observed: "up",
          verified: true,
          surface: "operator_ui",
        })
      )
    );
    const publicOperator = exposure.rows.find(row => row.id === "public-operator-ui");
    const onSetSurface = renderSection(exposure);

    fireEvent.click(screen.getByTestId("gateway-exposure-public-operator-ui-switch"));

    expect(onSetSurface).toHaveBeenCalledWith(publicOperator, false, false);
    expect(screen.queryByTestId("gateway-public-consent-dialog")).not.toBeInTheDocument();
  });
});
