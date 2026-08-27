// Invariant: Catalog chrome keeps Activity one click away, including error and
// no-workspace. Owning layer: Agents catalog location. Canonical suite: this file.
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AgentsCatalogLocation } from "../agents-catalog-location";
import { useAgentsFleetPage } from "../use-agents-catalog";

vi.mock("../agents-app-tabs", () => ({
  AgentsAppTabs: ({ value }: { value: string }) => (
    <div data-testid="agents-app-tabs" data-value={value} />
  ),
}));

vi.mock("../use-agents-catalog", () => ({
  useAgentsFleetPage: vi.fn(),
}));

vi.mock("@compozy/ui", async importOriginal => {
  const actual = await importOriginal<typeof import("@compozy/ui")>();
  return { ...actual, useTopbarSlot: () => undefined };
});

const useAgentsFleetPageMock = vi.mocked(useAgentsFleetPage);

function catalogPage(overrides: Partial<ReturnType<typeof useAgentsFleetPage>> = {}) {
  return {
    workspaceId: "ws_test",
    catalogQuery: {},
    agents: [],
    fleetTotal: 0,
    rows: [],
    categoryOptions: [],
    search: {},
    draftQuery: "",
    searchInputRef: { current: null },
    view: "rows" as const,
    setDraftQuery: vi.fn(),
    setFilters: vi.fn(),
    setView: vi.fn(),
    clearFilters: vi.fn(),
    openCreate: vi.fn(),
    openNewSession: vi.fn(),
    newSessionDisabled: false,
    isLoading: false,
    isFirstRunEmpty: false,
    isFilteredEmpty: false,
    sessionsPartial: false,
    hasMore: false,
    isLoadingMore: false,
    loadMore: vi.fn(),
    showFacets: false,
    showViewToggle: false,
    agentsError: null,
    retryAgents: vi.fn(),
    ...overrides,
  } as ReturnType<typeof useAgentsFleetPage>;
}

describe("AgentsCatalogLocation", () => {
  beforeEach(() => {
    useAgentsFleetPageMock.mockReset();
  });

  it("Should keep the Activity tab on Catalog when no workspace is selected", () => {
    useAgentsFleetPageMock.mockReturnValue(catalogPage({ workspaceId: "" }));

    render(<AgentsCatalogLocation search={{}} />);

    expect(screen.getByTestId("agents-app-tabs")).toHaveAttribute("data-value", "catalog");
    expect(screen.getByTestId("agents-no-workspace")).toHaveTextContent("No workspace selected");
  });

  it("Should keep the Activity tab when the catalog request fails", () => {
    useAgentsFleetPageMock.mockReturnValue(
      catalogPage({ agentsError: new Error("agents unavailable") })
    );

    render(<AgentsCatalogLocation search={{}} />);

    expect(screen.getByTestId("agents-app-tabs")).toHaveAttribute("data-value", "catalog");
    expect(screen.getByTestId("agent-fleet-error")).toHaveTextContent("Couldn't load agents");
  });
});
