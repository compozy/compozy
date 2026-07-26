import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LiveBadge } from "../live-badge";

describe("LiveBadge", () => {
  it("Should render the default live label with the success tone", () => {
    render(<LiveBadge />);
    const badge = screen.getByRole("status");
    expect(badge).toHaveTextContent("Live");
    expect(badge).toHaveAttribute("data-tone", "success");
    expect(badge).toHaveAttribute("aria-live", "polite");
  });

  it("Should render a custom label and tone without pulse", () => {
    render(<LiveBadge data-testid="live-badge" label="Stale" pulse={false} tone="neutral" />);
    const badge = screen.getByTestId("live-badge");
    expect(badge).toHaveTextContent("Stale");
    expect(badge).toHaveAttribute("data-tone", "neutral");
    const dot = badge.querySelector('[data-slot="pill-dot"]');
    expect(dot).not.toBeNull();
    expect(dot).not.toHaveAttribute("data-pulse", "true");
  });
});
