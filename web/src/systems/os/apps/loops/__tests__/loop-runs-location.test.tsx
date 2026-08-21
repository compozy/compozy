// Suite: Loop Runs route composition
// Invariant: an inventory request failure renders a recoverable error, never a successful empty state.
// Boundary IN: LoopRunsLocation query-state routing and retry wiring.
// Boundary OUT: inventory rows and query transport, owned by their component and adapter suites.

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
}));

vi.mock("@compozy/ui", async importOriginal => {
  const actual = await importOriginal<typeof import("@compozy/ui")>();
  return { ...actual, useTopbarSlot: vi.fn() };
});

vi.mock("../use-loop-runs-route", () => ({
  useLoopRunsRoute: useLoopRunsRouteMock,
}));

vi.mock("@/systems/loops", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/loops")>();
  return actual;
});

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
});
