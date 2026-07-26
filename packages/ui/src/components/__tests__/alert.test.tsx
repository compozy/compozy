import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Alert, AlertActions, AlertDescription, AlertMeta, AlertTitle } from "../alert";

describe("Alert", () => {
  it("renders role=alert by default and forwards class + data-variant", () => {
    render(
      <Alert data-testid="alert" variant="warning" className="ring-1">
        <AlertTitle>Heads up</AlertTitle>
        <AlertDescription>Restart required</AlertDescription>
      </Alert>
    );
    const alert = screen.getByTestId("alert");
    expect(alert).toHaveAttribute("role", "alert");
    expect(alert).toHaveAttribute("data-variant", "warning");
    expect(alert).toHaveAttribute("data-slot", "alert");
    expect(alert).toHaveClass("ring-1");
  });

  it("accepts a role override for non-danger tones (e.g. status)", () => {
    render(
      <Alert data-testid="alert" variant="success" role="status">
        <AlertTitle>OK</AlertTitle>
      </Alert>
    );
    expect(screen.getByTestId("alert")).toHaveAttribute("role", "status");
  });

  it("Should support the new semantic variants (success/warning/info/accent/danger)", () => {
    for (const variant of ["success", "warning", "info", "accent", "danger"] as const) {
      const { unmount } = render(
        <Alert data-testid={`alert-${variant}`} variant={variant}>
          <AlertTitle>ok</AlertTitle>
        </Alert>
      );
      expect(screen.getByTestId(`alert-${variant}`)).toHaveAttribute("data-variant", variant);
      unmount();
    }
  });

  it("Should render meta and actions slots after the description", () => {
    render(
      <Alert data-testid="alert" variant="warning">
        <AlertTitle>Provider missing</AlertTitle>
        <AlertDescription>Reconnect the provider.</AlertDescription>
        <AlertMeta data-testid="alert-meta">session sess_123</AlertMeta>
        <AlertActions data-testid="alert-actions">
          <button type="button">Retry</button>
        </AlertActions>
      </Alert>
    );

    expect(screen.getByTestId("alert-meta")).toHaveAttribute("data-slot", "alert-meta");
    expect(screen.getByTestId("alert-meta").className).toContain("text-form-hint");
    expect(screen.getByTestId("alert-meta").className).not.toContain("eyebrow");
    expect(screen.getByText("Reconnect the provider.").className).toContain(
      "group-has-[>svg]/alert:col-start-2"
    );
    expect(screen.getByTestId("alert").className).toContain("gap-y-2");
    expect(screen.getByTestId("alert").className).toContain("row-span-full");
    expect(screen.getByTestId("alert-actions")).toHaveAttribute("data-slot", "alert-actions");
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(
      screen
        .getByText("Reconnect the provider.")
        .compareDocumentPosition(screen.getByTestId("alert-meta")) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
    expect(
      screen
        .getByTestId("alert-meta")
        .compareDocumentPosition(screen.getByTestId("alert-actions")) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
  });
});
