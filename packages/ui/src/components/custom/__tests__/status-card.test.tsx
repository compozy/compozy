import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "../../button";
import { StatusCard } from "../status-card";

describe("StatusCard", () => {
  it("Should compose header, body, footer, and action slots", () => {
    render(
      <StatusCard data-testid="card" tone="warning">
        <StatusCard.Header label="Degraded" />
        <StatusCard.Body>Daemon responded with warnings.</StatusCard.Body>
        <StatusCard.Footer>
          <StatusCard.Action>
            <Button data-testid="action" size="sm" type="button">
              Retry
            </Button>
          </StatusCard.Action>
        </StatusCard.Footer>
      </StatusCard>
    );

    const card = screen.getByTestId("card");
    const dot = card.querySelector('[data-slot="status-card-dot"]');
    const label = card.querySelector('[data-slot="status-card-label"]');
    const body = card.querySelector('[data-slot="status-card-body"]');
    const footer = card.querySelector('[data-slot="status-card-footer"]');
    const action = screen.getByTestId("action");

    expect(card).toHaveAttribute("data-tone", "warning");
    expect(dot).toHaveAttribute("data-tone", "warning");
    expect(label).toHaveTextContent("Degraded");
    expect(body).toHaveTextContent("Daemon responded with warnings.");
    expect(footer).toBeInTheDocument();
    action.focus();
    expect(action).toHaveFocus();
  });

  it.each(["success", "warning", "danger", "info", "neutral"] as const)(
    "Should propagate %s tone to the status dot",
    tone => {
      render(
        <StatusCard data-testid="card" tone={tone}>
          <StatusCard.Header label={tone} />
        </StatusCard>
      );

      expect(screen.getByTestId("card")).toHaveAttribute("data-tone", tone);
      expect(
        screen.getByTestId("card").querySelector('[data-slot="status-card-dot"]')
      ).toHaveAttribute("data-tone", tone);
    }
  );
});
