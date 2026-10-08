// Suite: Automations catalog location
// Invariant: the window layer turns the listing model into the right chrome — first-run
//   starts and suggestions without a toolbar, the runtime-off alert with its way to Settings,
//   the partial-failure alert naming the failed kind with its "—" count, and the footer truth.
// Boundary IN: the useAutomationsPage model (mocked) and the route search.
// Boundary OUT: data loading and merging (use-automations-page suite), rows (listing suite).
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { automationStoryJobs } from "@/systems/automation/mocks";
import { toAutomationView } from "@/systems/automation";
import { scopedListingScopeFixture } from "@/systems/profiles/mocks";

const state = vi.hoisted(() => ({
  page: {} as Record<string, unknown>,
  slot: null as null | { count?: ReactNode; toolbar?: ReactNode; actions?: ReactNode },
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    search,
    to,
    ...props
  }: {
    children?: ReactNode;
    search?: Record<string, string>;
    to?: string;
  }) => (
    <a href={`${to}${search ? `?${new URLSearchParams(search).toString()}` : ""}`} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("@compozy/ui", async importOriginal => ({
  ...(await importOriginal<typeof import("@compozy/ui")>()),
  useTopbarSlot: (slot: typeof state.slot) => {
    state.slot = slot;
  },
}));

vi.mock("@/systems/automation", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/automation")>()),
  AutomationEditorDialog: () => null,
  AutomationSuggestionsPanel: ({ workspaceID }: { workspaceID: string }) => (
    <div data-testid="suggestions-panel">{workspaceID}</div>
  ),
}));

vi.mock("../../automation/use-automation-page", () => ({
  useAutomationsPage: () => state.page,
}));

const { AutomationsCatalogLocation } = await import("../automations-catalog-location");

const storyViews = automationStoryJobs.map(job => toAutomationView(job));

function page(overrides: Record<string, unknown> = {}) {
  return {
    canLoadMore: false,
    clearFilters: vi.fn(),
    confirmDelete: vi.fn(),
    copyLink: vi.fn(),
    counts: { all: 7, schedule: 4, event: 3 },
    create: vi.fn(),
    deletePending: false,
    deleteTarget: null,
    edit: vi.fn(),
    editorDialogProps: {},
    enabledCount: 0,
    enabledFilter: null,
    firstRun: false,
    hasActiveFilters: false,
    isFetchingMore: false,
    isLoading: false,
    isPaused: false,
    isRunPending: () => false,
    isTogglePending: () => false,
    items: [],
    loadError: null,
    loadMore: vi.fn(),
    loopFilter: null,
    nextRunAt: null,
    partialFailure: null,
    profileScope: scopedListingScopeFixture,
    retry: vi.fn(),
    runNow: vi.fn(),
    scopeFilter: null,
    searchQuery: "",
    setDeleteTarget: vi.fn(),
    setEnabledFilter: vi.fn(),
    setLoopFilter: vi.fn(),
    setScopeFilter: vi.fn(),
    setSearchQuery: vi.fn(),
    setSourceFilter: vi.fn(),
    setStart: vi.fn(),
    setTargetFilter: vi.fn(),
    setView: vi.fn(),
    sourceFilter: null,
    start: null,
    suggestionsWorkspaceId: null,
    targetFilter: null,
    toggleEnabled: vi.fn(),
    total: 7,
    unavailableMessage: null,
    view: "rows",
    ...overrides,
  };
}

function renderLocation(overrides: Record<string, unknown> = {}) {
  state.page = page(overrides);
  render(<AutomationsCatalogLocation search={{}} />);
}

function renderToolbar() {
  const toolbar = state.slot?.toolbar;
  if (!toolbar) throw new Error("Expected the listing toolbar");
  render(<>{toolbar}</>);
}

describe("AutomationsCatalogLocation", () => {
  beforeEach(() => {
    state.slot = null;
  });

  it("Should open first run with two starts, suggestions and no toolbar", () => {
    renderLocation({
      counts: { all: 0, schedule: 0, event: 0 },
      firstRun: true,
      suggestionsWorkspaceId: "ws_launch_hq",
      total: 0,
    });

    expect(screen.getByTestId("automations-empty-start-schedule")).toHaveAttribute(
      "href",
      "/automations?create=1&start=schedule"
    );
    expect(screen.getByTestId("automations-empty-start-event")).toHaveAttribute(
      "href",
      "/automations?create=1&start=event"
    );
    expect(screen.getByTestId("suggestions-panel")).toHaveTextContent("ws_launch_hq");
    expect(state.slot?.toolbar).toBeUndefined();
    expect(state.slot?.count).toBe(0);
  });

  it("Should point the runtime-off alert at Settings › Automation", () => {
    renderLocation({
      unavailableMessage:
        "CompozyOS couldn't load your automations right now. Try again in a moment.",
    });

    const alert = screen.getByTestId("automations-runtime-alert");
    expect(alert).toHaveTextContent("Automations aren't available right now");
    expect(within(alert).getByRole("link", { name: "Open Settings" })).toHaveAttribute(
      "href",
      "/settings/automation"
    );
  });

  it("Should name the failed kind above the loaded rows and blank its view count", async () => {
    const retry = vi.fn();
    renderLocation({
      counts: { all: null, schedule: 4, event: null },
      enabledCount: 3,
      items: storyViews,
      partialFailure: "event",
      retry,
      total: null,
    });

    const alert = screen.getByTestId("automations-partial-alert");
    expect(alert).toHaveTextContent("Couldn't load event automations.");
    await userEvent.click(within(alert).getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(screen.getByTestId("automations-list-rows")).toBeVisible();
    expect(state.slot?.count).toBe("—");

    renderToolbar();
    expect(screen.getByTestId("automation-start-event")).toHaveTextContent("On events —");
    expect(screen.getByTestId("automation-start-all")).toHaveTextContent("All —");
    expect(screen.getByTestId("automation-start-schedule")).toHaveTextContent("Scheduled 4");
  });

  it("Should sum the footer across both kinds with the soonest next run", () => {
    renderLocation({
      enabledCount: 6,
      items: storyViews,
      nextRunAt: new Date(Date.now() + 14 * 3_600_000 + 60_000).toISOString(),
      total: 7,
    });

    expect(screen.getByTestId("automations-list-footer")).toHaveTextContent(
      "7 automations · 6 on · next run in 14h"
    );
  });
});
