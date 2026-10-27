// Suite: Loop Runs route composition
// Invariant: a read failure renders a recoverable degraded state over the last
// read, never a successful empty state — for the inventory and the runs roster alike.
// A successful filtered-empty roster explains the active filters and clears them together.
// Under All projects each run names its project and the one-project Steps view is not offered.
// Boundary IN: LoopRunsLocation query-state routing and retry wiring.
// Boundary OUT: inventory rows and query transport, owned by their component and adapter suites.

import { createElement, type ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useTopbarSlot } from "@compozy/ui";
import { scopedListingScopeFixture } from "@/systems/profiles/mocks";
import type { LoopOutcomeValue, LoopRunsRouteSearch } from "@/systems/loops";

const { inventoryRefetch, useLoopRunsRouteMock } = vi.hoisted(() => ({
  inventoryRefetch: vi.fn(),
  useLoopRunsRouteMock: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
  // Rows render real links; the route pattern is what the assertion cares about.
  Link: ({
    to,
    search,
    children,
    ...props
  }: {
    to: string;
    search?: Record<string, unknown>;
    children?: ReactNode;
  }) =>
    createElement(
      "a",
      { href: to, "data-search": search ? JSON.stringify(search) : undefined, ...props },
      children
    ),
}));

vi.mock("@compozy/ui", async importOriginal => {
  const actual = await importOriginal<typeof import("@compozy/ui")>();
  return { ...actual, useTopbarSlot: vi.fn() };
});

vi.mock("../use-loop-runs-route", () => ({
  useLoopRunsRoute: useLoopRunsRouteMock,
}));

const { LoopRunsLocation } = await import("../loop-runs-location");

const projectFilterFixture = {
  visible: false,
  values: [],
  field: null,
  setValues: vi.fn(),
  clear: vi.fn(),
};

describe("LoopRunsLocation", () => {
  beforeEach(() => {
    inventoryRefetch.mockReset();
    vi.mocked(useTopbarSlot).mockClear();
    useLoopRunsRouteMock.mockReturnValue({
      profile: scopedListingScopeFixture,
      outcome: "all",
      runsQuery: { data: { runs: [] }, isLoading: false, error: null },
      setOriginFilter: vi.fn(),
      setOutcome: vi.fn(),
      ready: true,
      allProjects: false,
      projectFilter: projectFilterFixture,
      projectOf: undefined,
      inventoryState: "waiting",
      loopOptions: ["catalog-only-loop"],
      inventory: {
        items: [],
        loadedCount: 0,
        hasMore: false,
        isLoading: false,
        isFetchingNextPage: false,
        isError: true,
        error: new Error("Transport unavailable"),
        fetchNextPage: vi.fn(),
        refetch: inventoryRefetch,
      },
      runOptions: [],
      setInventoryState: vi.fn(),
      setInventoryLoop: vi.fn(),
      setInventoryRun: vi.fn(),
      setInventoryView: vi.fn(),
      clearInventoryFilters: vi.fn(),
    });
  });

  it("Should render a retryable error instead of an empty inventory when loading fails", async () => {
    const user = userEvent.setup();
    render(<LoopRunsLocation search={{ nodes: "waiting" }} />);

    expect(screen.getByText("Couldn't load steps")).toBeInTheDocument();
    expect(screen.queryByText("Nothing is waiting")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(inventoryRefetch).toHaveBeenCalledTimes(1);
  });

  it("Should publish a Loops parent crumb and a Runs leaf in the window head", () => {
    render(<LoopRunsLocation search={{ nodes: "waiting" }} />);

    expect(useTopbarSlot).toHaveBeenCalledWith(
      expect.objectContaining({
        crumb: "Runs",
        crumbs: [expect.objectContaining({ id: "loops", label: "Loops" })],
        onBack: expect.any(Function),
      })
    );
    const slot = vi.mocked(useTopbarSlot).mock.calls.at(-1)?.[0];
    expect(slot).not.toHaveProperty("glyph");
    expect(slot).not.toHaveProperty("count");
  });

  it.each([
    { search: { origin: "session" }, outcome: "all" },
    { search: { origin_session: "session-review" }, outcome: "all" },
    { search: { origin: "session", origin_session: "session-review" }, outcome: "done" },
  ] satisfies { search: LoopRunsRouteSearch; outcome: LoopOutcomeValue }[])(
    "Should recover from an empty roster filtered by $search and $outcome",
    async ({ search, outcome }) => {
      const user = userEvent.setup();
      const route = useLoopRunsRouteMock();
      useLoopRunsRouteMock.mockReturnValue({ ...route, inventoryState: undefined, outcome });

      render(<LoopRunsLocation search={search} />);

      expect(
        screen.getByRole("heading", { name: "No runs match this filter" })
      ).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Browse loops" })).not.toBeInTheDocument();
      await user.click(screen.getByRole("button", { name: "Clear filter" }));
      expect(route.setOriginFilter).toHaveBeenCalledExactlyOnceWith({});
      expect(route.setOutcome).toHaveBeenCalledExactlyOnceWith("all");
    }
  );

  it("Should populate the inventory filter from catalog options rather than run history", async () => {
    const user = userEvent.setup();
    const route = useLoopRunsRouteMock();
    useLoopRunsRouteMock.mockReturnValue({
      ...route,
      runsQuery: { data: { runs: [] }, isLoading: false, error: null },
      loopOptions: ["catalog-only-loop"],
      inventory: {
        items: [],
        loadedCount: 0,
        hasMore: false,
        isLoading: false,
        isFetchingNextPage: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        refetch: inventoryRefetch,
      },
    });
    render(<LoopRunsLocation search={{ nodes: "waiting" }} />);
    await user.click(screen.getByTestId("loop-node-inventory-loop-filter"));
    expect(await screen.findByText("catalog-only-loop")).toBeInTheDocument();
  });
  // Invariant: a read that has not answered is not an empty roster. The first read holds the
  // roster's own frame (its real column header over placeholder rows) in one named status region,
  // inside the page the rows will fill; a refresh behind rows already in hand never draws it.
  it("Should hold the roster's frame, not an empty state, while the first read is pending", () => {
    const route = useLoopRunsRouteMock();
    useLoopRunsRouteMock.mockReturnValue({
      ...route,
      inventoryState: undefined,
      runsQuery: { data: undefined, isPending: true, isLoading: false, error: null },
    });

    const { container, rerender } = render(<LoopRunsLocation search={{}} />);

    const pending = screen.getByTestId("loop-runs-loading");
    expect(pending).toHaveAttribute("role", "status");
    expect(pending).toHaveTextContent("Loading runs");
    for (const column of ["Loop", "Status", "Progress", "Started", "Duration"]) {
      expect(within(pending).getByText(column)).toBeInTheDocument();
    }
    expect(within(pending).queryByText("Project")).not.toBeInTheDocument();
    expect(container.querySelector('[data-slot="listing-page"]')).toContainElement(pending);
    expect(screen.queryByTestId("loop-runs-empty")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-runs-view")).not.toBeInTheDocument();

    // Under All projects the roster has its Project column, so the placeholder does too.
    useLoopRunsRouteMock.mockReturnValue({
      ...useLoopRunsRouteMock(),
      allProjects: true,
      projectOf: () => ({ kind: "user", name: "User" }),
    });
    rerender(<LoopRunsLocation search={{}} />);
    expect(within(screen.getByTestId("loop-runs-loading")).getByText("Project")).toBeVisible();
  });

  it("Should keep the rows it has while a refresh is in flight", () => {
    const route = useLoopRunsRouteMock();
    useLoopRunsRouteMock.mockReturnValue({
      ...route,
      inventoryState: undefined,
      runsQuery: {
        data: {
          runs: [
            {
              id: "looprun-77aa01b2c3d4e5f6",
              loop_name: "revisao-paralela",
              status: "done",
              generation: 1,
              created_at: "2026-08-19T18:40:00Z",
              attention: null,
              progress: { round: 1, steps_done: 6, steps_total: 6 },
            },
          ],
        },
        isPending: false,
        isFetching: true,
        isLoading: false,
        error: null,
      },
    });

    render(<LoopRunsLocation search={{}} />);

    expect(screen.getByText("revisao-paralela")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-runs-loading")).not.toBeInTheDocument();
  });

  it("Should hold the inventory's frame, not 'Nothing is waiting', while its first read is pending", () => {
    const route = useLoopRunsRouteMock();
    useLoopRunsRouteMock.mockReturnValue({
      ...route,
      inventory: {
        ...route.inventory,
        isError: false,
        error: null,
        isLoading: false,
        isPending: true,
      },
    });

    render(<LoopRunsLocation search={{ nodes: "waiting" }} />);

    const pending = screen.getByTestId("loop-node-inventory-loading");
    expect(pending).toHaveAttribute("role", "status");
    expect(pending).toHaveTextContent("Loading waiting steps");
    expect(within(pending).getByText("Time in state")).toBeInTheDocument();
    // The state tabs above are static, so they are the real ones and stay usable.
    expect(screen.getByTestId("loop-node-inventory-state-waiting")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-node-inventory-empty")).not.toBeInTheDocument();
  });

  // The window route reaches the page through a store, whose updates are never transitions; the
  // pane choice is deferred so React can render the swap as one and cross-fade it. What the swap
  // owes the reader is that the pane the route names is the pane on screen.
  it("Should end on the pane the route names after the Runs and Steps switch", async () => {
    const base = useLoopRunsRouteMock();
    const route = { ...base, inventory: { ...base.inventory, isError: false, error: null } };
    useLoopRunsRouteMock.mockReturnValue({ ...route, inventoryState: undefined });
    const { rerender } = render(<LoopRunsLocation search={{}} />);
    expect(screen.getByTestId("loop-runs")).toBeInTheDocument();

    useLoopRunsRouteMock.mockReturnValue({ ...route, inventoryState: "waiting" });
    rerender(<LoopRunsLocation search={{ nodes: "waiting" }} />);
    expect(await screen.findByTestId("loop-runs-inventory")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-runs")).not.toBeInTheDocument();

    useLoopRunsRouteMock.mockReturnValue({ ...route, inventoryState: undefined });
    rerender(<LoopRunsLocation search={{}} />);
    expect(await screen.findByTestId("loop-runs")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-runs-inventory")).not.toBeInTheDocument();
  });

  // task_05 requirement 5 / VC-36: transport-degraded never renders as empty.
  // The roster component implements the degraded notice; the route has to hand it
  // the failure and the retry instead of swallowing both behind its own state.
  it("Should keep the last-read runs and offer a retry when the runs read fails", async () => {
    const user = userEvent.setup();
    const runsRefetch = vi.fn();
    const route = useLoopRunsRouteMock();
    useLoopRunsRouteMock.mockReturnValue({
      ...route,
      inventoryState: undefined,
      runsQuery: {
        data: {
          runs: [
            {
              id: "looprun-77aa01b2c3d4e5f6",
              loop_name: "revisao-paralela",
              status: "running",
              generation: 1,
              started_at: "2026-08-19T18:40:00Z",
              attention: null,
              progress: { round: 1, steps_done: 2, steps_total: 6 },
            },
          ],
        },
        isLoading: false,
        error: new Error("Transport unavailable"),
        refetch: runsRefetch,
      },
    });

    render(<LoopRunsLocation search={{}} />);

    // The rows survive the failure: they are the last good read, not fresh truth.
    expect(screen.getByTestId("loop-runs-view")).toBeInTheDocument();
    expect(screen.getByText("revisao-paralela")).toBeInTheDocument();
    expect(screen.getByTestId("loop-runs-degraded")).toBeInTheDocument();
    expect(screen.queryByText("No runs yet")).not.toBeInTheDocument();

    expect(screen.getByTestId("loop-runs-degraded")).toHaveAttribute("data-cause", "read-failed");
    await user.click(screen.getByTestId("loop-runs-degraded-retry"));
    expect(runsRefetch).toHaveBeenCalledTimes(1);
  });

  it("Should list All projects runs with their project and offer no Steps view", () => {
    const route = useLoopRunsRouteMock();
    useLoopRunsRouteMock.mockReturnValue({
      ...route,
      allProjects: true,
      inventoryState: undefined,
      projectFilter: {
        ...projectFilterFixture,
        visible: true,
        field: { key: "project", label: "Project", type: "multiselect", options: [] },
      },
      projectOf: (id: string) =>
        id === "ws_b" ? { kind: "project", name: "beta" } : { kind: "user", name: "User" },
      runsQuery: {
        data: {
          runs: [
            {
              id: "looprun-b",
              loop_name: "triage",
              status: "running",
              generation: 1,
              created_at: "2026-10-21T18:40:00Z",
              workspace_id: "ws_b",
              attention: null,
              progress: { round: 1, steps_done: 2, steps_total: 6 },
            },
          ],
        },
        isLoading: false,
        error: null,
      },
    });

    render(<LoopRunsLocation search={{ nodes: "waiting" }} />);
    const toolbar = vi.mocked(useTopbarSlot).mock.calls.at(-1)?.[0]?.toolbar;
    render(<>{toolbar}</>);

    expect(screen.getByTestId("loop-run-project")).toHaveTextContent("beta");
    // The row opens in place, bound to its own project.
    expect(screen.getByTestId("loop-run-name")).toHaveAttribute(
      "data-search",
      JSON.stringify({ workspace: "ws_b" })
    );
    expect(screen.queryByTestId("loop-runs-view-switch")).not.toBeInTheDocument();
  });

  // This roster is polled, not streamed, so "reconnecting" can only honestly mean
  // a failed read currently being retried. Telling a reader to wait for a retry
  // that has already settled into an error is a different failure than the error.
  it("Should call a retry in flight reconnecting, not a failed read", () => {
    const route = useLoopRunsRouteMock();
    useLoopRunsRouteMock.mockReturnValue({
      ...route,
      inventoryState: undefined,
      runsQuery: {
        data: { runs: [] },
        isLoading: false,
        // Already failed once, and asking again right now.
        isFetching: true,
        failureCount: 1,
        error: new Error("Transport unavailable"),
        refetch: vi.fn(),
      },
    });

    render(<LoopRunsLocation search={{}} />);

    const notice = screen.getByTestId("loop-runs-degraded");
    expect(notice).toHaveAttribute("data-cause", "reconnecting");
    expect(notice).toHaveTextContent("Reconnecting to CompozyOS.");
    // The two causes stay mutually exclusive; the read-failed sentence is absent.
    expect(notice).not.toHaveTextContent("could not be read");
  });
});
