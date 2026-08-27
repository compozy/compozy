// Invariant: Activity empty/error/loading are DataSurface states, and the
// teaching empty includes the CLI call verb. Owning layer: Activity location.
// Canonical suite: this file.
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AgentsActivityLocation } from "../agents-activity-location";
import { useAgentsActivity } from "../use-agents-activity";

vi.mock("../agents-app-tabs", () => ({
  AgentsAppTabs: ({ value }: { value: string }) => (
    <div data-testid="agents-app-tabs" data-value={value} />
  ),
}));

vi.mock("../use-agents-activity", () => ({
  useAgentsActivity: vi.fn(),
}));

vi.mock("@/systems/agent-comms", () => ({
  AgentCallTree: () => <div data-testid="agents-activity-tree" />,
}));

vi.mock("@compozy/ui", async importOriginal => {
  const actual = await importOriginal<typeof import("@compozy/ui")>();
  return { ...actual, useTopbarSlot: () => undefined };
});

const useAgentsActivityMock = vi.mocked(useAgentsActivity);

function activityPage(overrides: Partial<ReturnType<typeof useAgentsActivity>> = {}) {
  return {
    calls: [],
    tree: { groups: [] },
    total: 0,
    surface: { status: "empty" as const, emptyReason: "no-calls" as const },
    hasMore: false,
    loadingMore: false,
    loadMore: vi.fn(),
    refetch: vi.fn(),
    countsByRoot: new Map(),
    stale: false,
    openCall: vi.fn(),
    openCatalog: vi.fn(),
    stopSubtree: vi.fn(),
    drainFailure: null,
    drainOutcome: null,
    retryStopSubtree: vi.fn(),
    pendingStopRootSessionId: null,
    scope: {
      workspaceId: "ws_test",
      profileKey: "default",
      params: {},
      actingProfile: "default",
    },
    ...overrides,
  } as ReturnType<typeof useAgentsActivity>;
}

describe("AgentsActivityLocation", () => {
  beforeEach(() => {
    useAgentsActivityMock.mockReset();
  });

  it("Should teach the call verb in the empty hint and keep Catalog reachable", () => {
    useAgentsActivityMock.mockReturnValue(activityPage());

    render(<AgentsActivityLocation windowId="win_agents" />);

    expect(screen.getByTestId("agents-app-tabs")).toHaveAttribute("data-value", "activity");
    expect(screen.getByTestId("agents-activity-empty")).toHaveTextContent(
      "No agent is delegating work right now"
    );
    expect(screen.getByTestId("agents-activity-empty")).toHaveTextContent(
      `compozy call reviewer "…" --expect @contract.json`
    );
    expect(screen.getByTestId("agents-activity-empty")).not.toHaveTextContent(", live");
    expect(screen.getByRole("button", { name: "Browse agents" })).toBeInTheDocument();
  });

  it("Should route loading and error through DataSurface slots", () => {
    useAgentsActivityMock.mockReturnValue(
      activityPage({ surface: { status: "loading", emptyReason: null } })
    );
    const { rerender } = render(<AgentsActivityLocation windowId="win_agents" />);
    expect(screen.getByTestId("agents-activity-loading")).toBeInTheDocument();

    useAgentsActivityMock.mockReturnValue(
      activityPage({ surface: { status: "error", emptyReason: null } })
    );
    rerender(<AgentsActivityLocation windowId="win_agents" />);
    expect(screen.getByTestId("agents-activity-error")).toHaveTextContent("Couldn't load activity");
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("Should warn when live updates drop and page older rows without a loaded count", () => {
    useAgentsActivityMock.mockReturnValue(
      activityPage({
        surface: { status: "ready", emptyReason: null },
        hasMore: true,
        stale: true,
      })
    );

    render(<AgentsActivityLocation windowId="win_agents" />);

    expect(screen.getByTestId("agents-activity-stale")).toHaveTextContent(
      "Live updates disconnected"
    );
    expect(screen.getByTestId("agents-activity-more")).toHaveTextContent("Load older");
    expect(screen.getByTestId("agents-activity-page")).not.toHaveTextContent(/of \d+ loaded/);
  });
});
