// Suite: Loop Runs route composition
// Invariant: a read failure renders a recoverable degraded state over the last
// read, never a successful empty state — for the inventory and the runs roster alike.
// Boundary IN: LoopRunsLocation query-state routing and retry wiring.
// Boundary OUT: inventory rows and query transport, owned by their component and adapter suites.

import { createElement, type ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useTopbarSlot } from "@compozy/ui";

const { inventoryRefetch, useLoopRunsRouteMock } = vi.hoisted(() => ({
  inventoryRefetch: vi.fn(),
  useLoopRunsRouteMock: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
  // Rows render real links; the route pattern is what the assertion cares about.
  Link: ({ to, children, ...props }: { to: string; children?: ReactNode }) =>
    createElement("a", { href: to, ...props }, children),
}));

vi.mock("@compozy/ui", async importOriginal => {
  const actual = await importOriginal<typeof import("@compozy/ui")>();
  return { ...actual, useTopbarSlot: vi.fn() };
});

vi.mock("../use-loop-runs-route", () => ({
  useLoopRunsRoute: useLoopRunsRouteMock,
}));

const { LoopRunsLocation } = await import("../loop-runs-location");

describe("LoopRunsLocation", () => {
  beforeEach(() => {
    inventoryRefetch.mockReset();
    vi.mocked(useTopbarSlot).mockClear();
    useLoopRunsRouteMock.mockReturnValue({
      outcome: "all",
      runsQuery: { data: { runs: [] }, isLoading: false, error: null },
      setOriginFilter: vi.fn(),
      setOutcome: vi.fn(),
      workspaceId: "workspace-1",
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

    expect(screen.getByText("Unable to load node inventory")).toBeInTheDocument();
    expect(screen.queryByText("Nothing is waiting")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry inventory" }));
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
    expect(notice).toHaveTextContent("Reconnecting to the daemon.");
    // The two causes stay mutually exclusive; the read-failed sentence is absent.
    expect(notice).not.toHaveTextContent("could not be read");
  });
});
