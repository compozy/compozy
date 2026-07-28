import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ConnectionIndicator, type ConnectionStatus } from "../connection-indicator";

const EXPECTED: Record<ConnectionStatus, { label: string; tone: string; pulse: string }> = {
  connected: { label: "Connected", tone: "success", pulse: "false" },
  connecting: { label: "Connecting", tone: "warning", pulse: "true" },
  disconnected: { label: "Disconnected", tone: "danger", pulse: "false" },
  error: { label: "Connection error", tone: "danger", pulse: "false" },
};

describe("ConnectionIndicator", () => {
  it.each<ConnectionStatus>(["connected", "connecting", "disconnected", "error"])(
    "Should render %s with the canonical tone and label",
    status => {
      render(<ConnectionIndicator data-testid="indicator" status={status} />);

      const expected = EXPECTED[status];
      const indicator = screen.getByTestId("indicator");
      const dot = indicator.querySelector('[data-slot="connection-indicator-dot"]');
      const label = indicator.querySelector('[data-slot="connection-indicator-label"]');

      expect(indicator).toHaveAttribute("data-status", status);
      expect(indicator).toHaveAttribute("role", "status");
      expect(indicator).toHaveAttribute("aria-live", "polite");
      expect(dot).toHaveAttribute("aria-hidden", "true");
      expect(dot).toHaveAttribute("data-tone", expected.tone);
      if (expected.pulse === "true") {
        expect(dot).toHaveAttribute("data-pulse", "true");
      } else {
        expect(dot).not.toHaveAttribute("data-pulse");
      }
      expect(label).toHaveTextContent(expected.label);
    }
  );

  it("Should support compound Dot and Label slots", () => {
    render(
      <ConnectionIndicator data-testid="indicator" status="connected">
        <ConnectionIndicator.Dot data-testid="dot" />
        <ConnectionIndicator.Label data-testid="label">Daemon ready</ConnectionIndicator.Label>
      </ConnectionIndicator>
    );

    expect(screen.getByTestId("dot")).toHaveAttribute("data-tone", "success");
    expect(screen.getByTestId("label")).toHaveTextContent("Daemon ready");
  });

  it("Should default the variant to 'footer' rendering both dot and label", () => {
    render(<ConnectionIndicator data-testid="indicator" status="connected" />);
    const indicator = screen.getByTestId("indicator");
    expect(indicator).toHaveAttribute("data-variant", "footer");
    expect(indicator.querySelector('[data-slot="connection-indicator-dot"]')).not.toBeNull();
    expect(indicator.querySelector('[data-slot="connection-indicator-label"]')).not.toBeNull();
  });

  it("Should render only the dot when variant='rail-dot'", () => {
    render(<ConnectionIndicator data-testid="indicator" status="connected" variant="rail-dot" />);
    const indicator = screen.getByTestId("indicator");
    expect(indicator).toHaveAttribute("data-variant", "rail-dot");
    expect(indicator.querySelector('[data-slot="connection-indicator-dot"]')).not.toBeNull();
    expect(indicator.querySelector('[data-slot="connection-indicator-label"]')).toBeNull();
  });
});
