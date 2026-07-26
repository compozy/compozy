import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { BridgeListPanel } from "@/systems/bridges/components/bridge-list-panel";
import type { BridgeHealthMap, BridgeSummary } from "@/systems/bridges/types";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
    <a
      data-params={JSON.stringify(params)}
      href={typeof to === "string" ? to : "#"}
      {...(props as Record<string, unknown>)}
    >
      {children as React.ReactNode}
    </a>
  ),
}));

function makeBridge(overrides: Partial<BridgeSummary> = {}): BridgeSummary {
  return {
    created_at: "2026-04-13T12:00:00Z",
    delivery_defaults: {},
    display_name: "Support",
    dm_policy: "open",
    enabled: true,
    extension_name: "ext-telegram",
    id: "brg_support",
    notification_suppress: false,
    platform: "telegram",
    provider_config: {},
    routing_policy: { include_group: true, include_peer: true, include_thread: true },
    scope: "workspace",
    status: "ready",
    updated_at: "2026-04-13T12:30:00Z",
    workspace_id: "ws_test",
    ...overrides,
  };
}

function makeHealthMap(entries: Record<string, BridgeHealthMap[string]>): BridgeHealthMap {
  return entries;
}

const defaultProps = {
  bridgeHealth: {} as BridgeHealthMap,
  emptyState: "default" as const,
  onClearFilters: vi.fn(),
  view: "rows" as const,
};

describe("BridgeListPanel", () => {
  it("renders a flat sorted list of bridge rows with detail links", () => {
    const bridges = [
      makeBridge({ id: "brg_support", display_name: "Support", platform: "telegram" }),
      makeBridge({
        id: "brg_ops_email",
        display_name: "Ops email",
        extension_name: "ext-email",
        platform: "email",
      }),
      makeBridge({
        id: "brg_ops_slack",
        display_name: "Ops slack",
        extension_name: "ext-slack",
        platform: "slack",
        scope: "global",
        workspace_id: undefined,
      }),
    ];

    render(<BridgeListPanel {...defaultProps} bridges={bridges} />);

    expect(screen.getByTestId("bridge-list-rows")).toBeInTheDocument();
    expect(screen.getByTestId("bridge-item-brg_ops_slack")).toBeInTheDocument();
    expect(screen.getByTestId("bridge-item-brg_ops_email")).toBeInTheDocument();
    expect(screen.getByTestId("bridge-item-brg_support")).toBeInTheDocument();
    expect(screen.queryByTestId(/^bridge-list-group-header-/)).not.toBeInTheDocument();

    const link = screen.getByRole("link", { name: "Open Support" });
    expect(link).toHaveAttribute("href", "/bridges/$id");
    expect(link).toHaveAttribute("data-params", JSON.stringify({ id: "brg_support" }));
  });

  it("renders the server-provided page in canonical order without local filtering", () => {
    const bridges = [
      makeBridge({ id: "brg_support", display_name: "Support", platform: "telegram" }),
      makeBridge({
        id: "brg_ops_slack",
        display_name: "Ops slack",
        extension_name: "ext-slack",
        platform: "slack",
        status: "disabled",
      }),
    ];

    render(<BridgeListPanel {...defaultProps} bridges={bridges} />);
    expect(screen.getAllByTestId(/^bridge-item-/).map(row => row.dataset.bridge)).toEqual([
      "brg_support",
      "brg_ops_slack",
    ]);
  });

  it("renders cards view", () => {
    render(
      <BridgeListPanel
        {...defaultProps}
        bridges={[makeBridge({ id: "brg_support" })]}
        view="cards"
      />
    );

    expect(screen.getByTestId("bridge-list-card-grid")).toBeInTheDocument();
    expect(screen.getByTestId("bridge-card-brg_support")).toBeInTheDocument();
  });

  it("renders the filtered-empty state with clear filters action", async () => {
    const user = userEvent.setup();
    const onClearFilters = vi.fn();

    render(
      <BridgeListPanel
        {...defaultProps}
        bridges={[]}
        emptyState="filtered"
        onClearFilters={onClearFilters}
      />
    );

    const empty = screen.getByTestId("bridge-list-empty");
    expect(within(empty).getByText(/No bridges match/i)).toBeInTheDocument();
    await user.click(screen.getByTestId("bridge-list-clear-filters"));
    expect(onClearFilters).toHaveBeenCalled();
  });

  it("renders the default empty state when no bridges and no filters", () => {
    render(<BridgeListPanel {...defaultProps} bridges={[]} />);

    const empty = screen.getByTestId("bridge-list-empty");
    expect(within(empty).getByText(/No bridges yet/i)).toBeInTheDocument();
  });

  it("renders loading and error fallbacks", () => {
    const { rerender } = render(
      <BridgeListPanel {...defaultProps} bridges={[]} status="loading" />
    );

    expect(screen.getByTestId("bridge-list-loading")).toBeInTheDocument();

    rerender(<BridgeListPanel {...defaultProps} bridges={[]} errorMessage="boom" />);

    expect(screen.getByTestId("bridge-list-error")).toHaveTextContent("boom");
  });

  it("preserves the selected card geometry while bridges are loading", () => {
    render(<BridgeListPanel {...defaultProps} bridges={[]} status="loading" view="cards" />);

    expect(screen.getByRole("status", { name: "Loading bridges as cards" })).toBeInTheDocument();
    expect(
      screen.queryByRole("status", { name: "Loading bridges as rows" })
    ).not.toBeInTheDocument();
  });

  it("shows route count and status label from health", () => {
    render(
      <BridgeListPanel
        {...defaultProps}
        bridgeHealth={makeHealthMap({
          brg_support: {
            auth_failures_total: 0,
            bridge_instance_id: "brg_support",
            delivery_backlog: 1,
            delivery_dropped_total: 0,
            delivery_failures_total: 0,
            last_success_at: "2026-04-13T12:20:00Z",
            route_count: 2,
            status: "ready",
          },
        })}
        bridges={[makeBridge({ id: "brg_support" })]}
      />
    );

    expect(screen.getByText(/2 routes/i)).toBeInTheDocument();
    expect(screen.getByText("ready")).toBeInTheDocument();
    expect(screen.getByText("telegram")).toBeInTheDocument();
  });

  it("loads the next server page through an accessible explicit control", async () => {
    const user = userEvent.setup();
    const onLoadMore = vi.fn();
    const { rerender } = render(
      <BridgeListPanel
        {...defaultProps}
        bridges={[makeBridge()]}
        paginationStatus="available"
        onLoadMore={onLoadMore}
      />
    );

    await user.click(screen.getByRole("button", { name: "Load more bridges" }));
    expect(onLoadMore).toHaveBeenCalledOnce();

    rerender(
      <BridgeListPanel
        {...defaultProps}
        bridges={[makeBridge()]}
        paginationStatus="loading"
        onLoadMore={onLoadMore}
      />
    );
    expect(screen.getByTestId("bridge-list-load-more")).toBeDisabled();
    expect(screen.getByTestId("bridge-list-load-more")).toHaveAttribute("aria-busy", "true");
  });
});
