// @vitest-environment jsdom

import { useSelector } from "@xstate/store-react";
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { Profiler, useState, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  useNetworkThreads: vi.fn(),
  useNetworkDirects: vi.fn(),
  useActiveNetworkSession: vi.fn(),
}));

const emptyCatalog = {
  total: 0,
  hasMore: false,
  isLoading: false,
  isLoadingMore: false,
  loadMore: vi.fn().mockResolvedValue(undefined),
  error: null,
};

vi.mock("../use-threads", () => ({
  useNetworkThreads: mocks.useNetworkThreads,
}));
vi.mock("../use-directs", () => ({
  useNetworkDirects: mocks.useNetworkDirects,
}));
vi.mock("../use-active-session", () => ({
  useActiveNetworkSession: mocks.useActiveNetworkSession,
}));
vi.mock("../use-last-read", () => ({
  useLastRead: () => ({
    lastReadAt: vi.fn(() => null),
    markRead: vi.fn(),
  }),
}));

import { createNetworkChipFilter, useNetworkListFilters } from "../use-network-list-filters";
import { NetworkListFiltersProvider } from "../../contexts/network-list-filters-context";
import { useNetworkListFiltersStore } from "../use-network-list-filters-context";

function NetworkListFiltersWrapper({ children }: { children: ReactNode }) {
  return (
    <NetworkListFiltersProvider channel="ops" workspaceId="ws-url">
      {children}
    </NetworkListFiltersProvider>
  );
}

function ScopedFilterProbe({ workspaceId, channel }: { workspaceId: string; channel: string }) {
  const filters = useNetworkListFilters({ workspaceId, channel });
  const [transientOpen, setTransientOpen] = useState(false);

  return (
    <>
      <output data-testid="scoped-filter-fields">
        {filters.filters.map(filter => filter.field).join(",")}
      </output>
      <button
        onClick={() => filters.setFilters([createNetworkChipFilter("has_work")])}
        type="button"
      >
        Select work filter
      </button>
      <button onClick={() => setTransientOpen(true)} type="button">
        Open transient control
      </button>
      <output data-testid="transient-control-state">{transientOpen ? "open" : "closed"}</output>
    </>
  );
}

function ScopedFilterHarness({ workspaceId, channel }: { workspaceId: string; channel: string }) {
  return (
    <NetworkListFiltersProvider channel={channel} workspaceId={workspaceId}>
      <ScopedFilterProbe channel={channel} workspaceId={workspaceId} />
    </NetworkListFiltersProvider>
  );
}

const subscriberCommits = { filters: 0, search: 0 };

function SubscriberProfiler({
  id,
  children,
}: {
  id: keyof typeof subscriberCommits;
  children: ReactNode;
}) {
  return (
    <Profiler
      id={id}
      onRender={() => {
        subscriberCommits[id] += 1;
      }}
    >
      {children}
    </Profiler>
  );
}

function FilterSubscriber() {
  const store = useNetworkListFiltersStore();
  const filters = useSelector(store, snapshot => snapshot.context.filters);
  return <output data-testid="filter-subscriber">{filters.length}</output>;
}

function SearchSubscriber() {
  const store = useNetworkListFiltersStore();
  const searchQuery = useSelector(store, snapshot => snapshot.context.searchQuery);
  return <output data-testid="search-subscriber">{searchQuery}</output>;
}

function FilterEventControls() {
  const store = useNetworkListFiltersStore();

  return (
    <>
      <button
        onClick={() =>
          store.trigger.filtersChanged({ filters: [createNetworkChipFilter("has_work")] })
        }
        type="button"
      >
        Change filters
      </button>
      <button onClick={() => store.trigger.searchEntered({ searchQuery: "release" })} type="button">
        Enter search
      </button>
    </>
  );
}

function QueryStateEmitter() {
  const [queryRevision, setQueryRevision] = useState(0);

  return (
    <>
      <button onClick={() => setQueryRevision(revision => revision + 1)} type="button">
        Simulate query update
      </button>
      <output data-testid="query-revision">{queryRevision}</output>
    </>
  );
}

function SelectorHarness() {
  return (
    <NetworkListFiltersProvider channel="ops" workspaceId="ws-url">
      <FilterEventControls />
      <SubscriberProfiler id="filters">
        <FilterSubscriber />
      </SubscriberProfiler>
      <SubscriberProfiler id="search">
        <SearchSubscriber />
      </SubscriberProfiler>
      <QueryStateEmitter />
    </NetworkListFiltersProvider>
  );
}

describe("useNetworkListFilters", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    subscriberCommits.filters = 0;
    subscriberCommits.search = 0;
    mocks.useNetworkThreads.mockReturnValue({ ...emptyCatalog, threads: [] });
    mocks.useNetworkDirects.mockReturnValue({ ...emptyCatalog, directs: [] });
    mocks.useActiveNetworkSession.mockReturnValue({
      session: { peerId: "peer-url", sessionId: "session-url" },
      disabledReason: null,
      isLoading: false,
    });
  });

  it("Should push URL workspace, search, work, self-session, and sort to both server catalogs", async () => {
    const { result } = renderHook(
      () => useNetworkListFilters({ workspaceId: "ws-url", channel: "ops" }),
      { wrapper: NetworkListFiltersWrapper }
    );

    act(() => {
      result.current.setSearchQuery(" release ");
      result.current.setSort("alphabetical");
      result.current.setFilters([
        createNetworkChipFilter("has_work"),
        createNetworkChipFilter("includes_me"),
      ]);
    });

    await waitFor(() => {
      expect(mocks.useNetworkThreads).toHaveBeenLastCalledWith("ops", {
        workspaceId: "ws-url",
        enabled: true,
        query: {
          query: "release",
          has_work: true,
          session_id: "session-url",
          sort: "alphabetical",
        },
      });
    });
    expect(mocks.useNetworkDirects).toHaveBeenLastCalledWith("ops", {
      workspaceId: "ws-url",
      enabled: true,
      query: {
        query: "release",
        has_work: true,
        session_id: "session-url",
        sort: "alphabetical",
      },
    });
    expect(mocks.useActiveNetworkSession).toHaveBeenCalledWith("ops", {
      workspaceId: "ws-url",
    });
  });

  it("Should reset client filters when the workspace or channel identity changes", () => {
    const { rerender } = render(<ScopedFilterHarness channel="ops" workspaceId="ws-one" />);

    fireEvent.click(screen.getByRole("button", { name: "Select work filter" }));
    fireEvent.click(screen.getByRole("button", { name: "Open transient control" }));
    expect(screen.getByTestId("scoped-filter-fields")).toHaveTextContent("has_work");
    expect(screen.getByTestId("transient-control-state")).toHaveTextContent("open");

    rerender(<ScopedFilterHarness channel="ops" workspaceId="ws-two" />);
    expect(screen.getByTestId("scoped-filter-fields")).toHaveTextContent("");
    expect(screen.getByTestId("transient-control-state")).toHaveTextContent("open");

    rerender(<ScopedFilterHarness channel="build" workspaceId="ws-one" />);
    expect(screen.getByTestId("scoped-filter-fields")).toHaveTextContent("");
    expect(screen.getByTestId("transient-control-state")).toHaveTextContent("open");
  });

  it("Should commit only the selector changed by local controls or unrelated query state", () => {
    render(<SelectorHarness />);

    expect(subscriberCommits.filters).toBe(1);
    expect(subscriberCommits.search).toBe(1);

    fireEvent.click(screen.getByRole("button", { name: "Simulate query update" }));
    expect(screen.getByTestId("query-revision")).toHaveTextContent("1");
    expect(subscriberCommits.filters).toBe(1);
    expect(subscriberCommits.search).toBe(1);

    fireEvent.click(screen.getByRole("button", { name: "Enter search" }));
    expect(subscriberCommits.filters).toBe(1);
    expect(subscriberCommits.search).toBe(2);

    fireEvent.click(screen.getByRole("button", { name: "Change filters" }));
    expect(subscriberCommits.filters).toBe(2);
    expect(subscriberCommits.search).toBe(2);
  });
});
