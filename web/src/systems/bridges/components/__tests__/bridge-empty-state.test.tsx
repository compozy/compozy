import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { BridgeEmptyState } from "@/systems/bridges/components/bridge-empty-state";
import type { BridgeProvider } from "@/systems/bridges/types";

function makeProvider(overrides: Partial<BridgeProvider> = {}): BridgeProvider {
  return {
    config_schema: { schema: "provider-config", version: "2026-04-15" },
    description: "Provider-specific runtime settings",
    display_name: "Telegram",
    enabled: true,
    extension_name: "ext-telegram",
    health: "healthy",
    platform: "telegram",
    secret_slots: [],
    state: "active",
    ...overrides,
  };
}

describe("BridgeEmptyState", () => {
  it("renders the 'no providers' copy when the list is empty and disables create", () => {
    render(<BridgeEmptyState onCreate={vi.fn()} providers={[]} />);

    expect(screen.getByTestId("bridges-empty-state")).toBeInTheDocument();
    expect(screen.getByText(/No bridge providers installed/i)).toBeInTheDocument();
    expect(screen.getByTestId("bridge-empty-create-btn")).toBeDisabled();
  });

  it("renders the 'no bridges configured' copy with a CatalogCard per installed provider", async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();

    render(<BridgeEmptyState onCreate={onCreate} providers={[makeProvider()]} />);

    expect(screen.getByText(/No bridges configured/i)).toBeInTheDocument();
    expect(screen.getByText("Telegram")).toBeInTheDocument();
    const card = screen.getByTestId("bridge-provider-card-ext-telegram::telegram");
    expect(card).toHaveAttribute("data-slot", "catalog-card");
    expect(card.querySelector('[data-slot="catalog-card-logo"][data-size="lg"]')).not.toBeNull();

    await user.click(screen.getByTestId("bridge-empty-create-btn"));
    expect(onCreate).toHaveBeenCalledTimes(1);
  });

  it("disables create and shows unavailable copy when all providers are unhealthy", () => {
    render(
      <BridgeEmptyState
        onCreate={vi.fn()}
        providers={[makeProvider({ enabled: false, health: "unhealthy" })]}
      />
    );

    expect(screen.getByTestId("bridge-empty-create-btn")).toBeDisabled();
    expect(screen.getByText(/Installed providers are currently unavailable/i)).toBeInTheDocument();
  });
});
