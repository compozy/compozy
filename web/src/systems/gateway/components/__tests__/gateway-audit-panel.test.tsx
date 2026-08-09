// Suite: gateway self-audit panel
// Invariant: findings render in the daemon's own ranking with remediation that is actionable
// without reading the source, and "no findings" is an explicit result rather than an empty list.
// Owning layer: the gateway audit component.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { gatewayAuditFixture } from "../../mocks/fixtures";
import { GatewayAuditPanel } from "../gateway-audit-panel";

const FINDINGS = [
  {
    id: "gateway.exposure.refused",
    remediation: "Pair a device from this machine, then enable the private overlay again.",
    severity: "critical",
    summary: "Gateway exposure is refused by the active safety policy.",
  },
  {
    id: "gateway.public.operator_ui_exposed",
    remediation:
      "Run `compozy gateway surface disable operator_ui --tier public --generation 4` when public operator access is not required.",
    severity: "warning",
    summary: "The operator UI and management API are reachable on the public tier.",
    tier: "public",
  },
];

function renderPanel(overrides: Partial<React.ComponentProps<typeof GatewayAuditPanel>> = {}) {
  const props = {
    hasRun: true,
    isRunning: false,
    report: gatewayAuditFixture({ findings: FINDINGS, no_findings: false, local_only: false }),
    onRun: vi.fn(),
    ...overrides,
  };
  render(<GatewayAuditPanel {...props} />);
  return props;
}

describe("GatewayAuditPanel", () => {
  it("Should invite the first run before anything has been audited", () => {
    renderPanel({ hasRun: false, report: null });

    expect(screen.getByText("Audit has not run yet")).toBeInTheDocument();
    expect(screen.getByTestId("gateway-audit-run")).toHaveTextContent("Run audit");
  });

  it("Should run the audit on request", () => {
    const { onRun } = renderPanel({ hasRun: false, report: null });
    fireEvent.click(screen.getByTestId("gateway-audit-run"));
    expect(onRun).toHaveBeenCalledTimes(1);
  });

  it("Should render findings in the daemon's ranking", () => {
    renderPanel();
    const rendered = screen
      .getAllByTestId(/^gateway-audit-finding-/)
      .map(node => node.getAttribute("data-testid"));

    expect(rendered).toEqual([
      "gateway-audit-finding-gateway.exposure.refused",
      "gateway-audit-finding-gateway.public.operator_ui_exposed",
    ]);
  });

  it("Should show remediation that names the concrete next action", () => {
    renderPanel();

    expect(
      screen.getByTestId("gateway-audit-remediation-gateway.public.operator_ui_exposed")
    ).toHaveTextContent("compozy gateway surface disable operator_ui --tier public --generation 4");
  });

  it("Should carry severity beyond colour alone", () => {
    renderPanel();
    const finding = screen.getByTestId("gateway-audit-finding-gateway.exposure.refused");

    expect(finding).toHaveTextContent("Critical");
  });

  it("Should state no findings explicitly after a clean run", () => {
    renderPanel({ report: gatewayAuditFixture() });

    expect(screen.getByText("No findings")).toBeInTheDocument();
    expect(screen.queryAllByTestId(/^gateway-audit-finding-/)).toHaveLength(0);
    expect(screen.getByTestId("gateway-audit-run")).toHaveTextContent("Run again");
  });

  it("Should report a failed audit rather than implying a clean posture", () => {
    renderPanel({ error: "Failed to run the gateway audit: 500", report: null });

    expect(screen.getByRole("alert")).toHaveTextContent("Failed to run the gateway audit");
    expect(screen.queryByText("No findings")).not.toBeInTheDocument();
  });
});
