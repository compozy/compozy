import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/systems/status", () => ({
  useDaemonHealth: () => ({
    connectionStatus: "connected" as const,
    health: { status: "ok" },
    isInitialLoading: false,
  }),
}));

import { RuntimeConnectionIndicator } from "../connection-indicator";
import { resolveRuntimeConnectionState } from "../connection-indicator.logic";

describe("resolveRuntimeConnectionState", () => {
  it("Should return success solid when daemon is connected and healthy", () => {
    expect(resolveRuntimeConnectionState("connected", "ok")).toEqual({
      tone: "success",
      pulse: false,
      label: "Connected",
    });
  });

  it("Should return success pulse when daemon is connected but degraded", () => {
    expect(resolveRuntimeConnectionState("connected", "degraded")).toEqual({
      tone: "success",
      pulse: true,
      label: "Degraded",
    });
  });

  it("Should return success pulse when daemon is mid-connect", () => {
    expect(resolveRuntimeConnectionState("connecting", "ok")).toEqual({
      tone: "success",
      pulse: true,
      label: "Connecting",
    });
  });

  it("Should return danger solid when daemon is disconnected", () => {
    expect(resolveRuntimeConnectionState("disconnected", "ok")).toEqual({
      tone: "danger",
      pulse: false,
      label: "Disconnected",
    });
  });

  it("Should return danger solid with the error label when the query errors", () => {
    expect(resolveRuntimeConnectionState("error", undefined)).toEqual({
      tone: "danger",
      pulse: false,
      label: "Connection error",
    });
  });
});

describe("RuntimeConnectionIndicator", () => {
  it("Should render connected from a healthy canonical daemon response", () => {
    render(<RuntimeConnectionIndicator status="connected" />);
    const indicator = screen.getByTestId("runtime-connection-indicator");
    expect(indicator).toHaveAttribute("data-pulse", "false");
    expect(indicator).toHaveTextContent("Connected");
  });

  it("Should render the dot and label when variant defaults to footer", () => {
    render(<RuntimeConnectionIndicator status="connected" healthStatus="ok" />);
    const indicator = screen.getByTestId("runtime-connection-indicator");
    expect(indicator).toHaveAttribute("data-tone", "success");
    expect(indicator).toHaveAttribute("data-pulse", "false");
    expect(indicator).toHaveAttribute("data-status", "connected");
    expect(indicator).toHaveAttribute("data-variant", "footer");
    expect(indicator).toHaveAttribute("role", "status");
    expect(indicator.querySelector('[data-slot="connection-indicator-dot"]')).not.toBeNull();
    expect(indicator.querySelector('[data-slot="connection-indicator-label"]')).toHaveTextContent(
      "Connected"
    );
  });

  it("Should render only the dot when dotOnly is true (rail mode)", () => {
    render(<RuntimeConnectionIndicator status="disconnected" healthStatus="ok" dotOnly />);
    const indicator = screen.getByTestId("runtime-connection-indicator");
    expect(indicator).toHaveAttribute("data-variant", "rail-dot");
    expect(indicator.querySelector('[data-slot="connection-indicator-dot"]')).not.toBeNull();
    expect(indicator.querySelector('[data-slot="connection-indicator-label"]')).toBeNull();
  });

  it("Should pulse the dot when daemon is connected but degraded", () => {
    render(<RuntimeConnectionIndicator status="connected" healthStatus="degraded" />);
    const indicator = screen.getByTestId("runtime-connection-indicator");
    expect(indicator).toHaveAttribute("data-tone", "success");
    expect(indicator).toHaveAttribute("data-pulse", "true");
    const dot = indicator.querySelector('[data-slot="connection-indicator-dot"]');
    expect(dot).toHaveAttribute("data-pulse", "true");
  });

  it("Should paint the danger tone when daemon is unreachable", () => {
    render(<RuntimeConnectionIndicator status="disconnected" healthStatus="ok" />);
    const indicator = screen.getByTestId("runtime-connection-indicator");
    expect(indicator).toHaveAttribute("data-tone", "danger");
    expect(indicator).toHaveAttribute("data-pulse", "false");
  });
});
