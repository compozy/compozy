import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

const { LoopCatalog } = await import("../catalog/loop-catalog");
const { loopCatalogFixtures } = await import("../../mocks/fixtures");
const reviewCatalogEntry = loopCatalogFixtures.find(entry => entry.name === "review-and-fix")!;
type LoopCatalogEntry = import("../../types").LoopCatalogEntry;
type ListingViewMode = import("@compozy/ui").ListingViewMode;

function Harness({
  onRun,
  onClearFilters = () => {},
  entries = loopCatalogFixtures,
  facets,
  hasActiveFilters = false,
  hasNextPage = false,
  isFetchingNextPage = false,
  onLoadMore,
  view = "rows",
}: {
  onRun: (entry: LoopCatalogEntry) => void;
  onClearFilters?: () => void;
  entries?: readonly LoopCatalogEntry[];
  facets?: import("../../types").LoopsListResponse["facets"];
  hasActiveFilters?: boolean;
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  onLoadMore?: () => void;
  view?: ListingViewMode;
}) {
  return (
    <LoopCatalog
      entries={entries}
      facets={facets}
      hasActiveFilters={hasActiveFilters}
      hasNextPage={hasNextPage}
      isFetchingNextPage={isFetchingNextPage}
      onClearFilters={onClearFilters}
      onLoadMore={onLoadMore}
      onRun={onRun}
      view={view}
    />
  );
}

describe("LoopCatalog", () => {
  it("Should render the bundled group with success rate and the last-outcome pill", () => {
    render(<Harness onRun={() => {}} />);
    expect(screen.getByTestId("loop-group-read-only")).toHaveTextContent("Built-in");
    expect(screen.queryByTestId("loop-group-workspace")).not.toBeInTheDocument();
    expect(screen.getByText("90%")).toBeInTheDocument();
    expect(screen.getByText("100%")).toBeInTheDocument();
    expect(screen.getAllByText("Running")).toHaveLength(2);
    expect(screen.queryByTestId("loop-catalog-best")).not.toBeInTheDocument();
  });

  it("Should show the canceled terminal on a roster row without offering a stop control", () => {
    const canceledEntry: LoopCatalogEntry = {
      ...loopCatalogFixtures[0],
      last_run: { ...loopCatalogFixtures[0].last_run!, status: "canceled" },
    };
    render(<Harness entries={[canceledEntry]} onRun={() => {}} />);
    const row = screen.getByTestId("loop-catalog-row");
    expect(within(row).getByText("Canceled")).toBeInTheDocument();
    expect(within(row).queryByText(/stop/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /stop/i })).not.toBeInTheDocument();
  });

  it("Should not claim sampled catalog-wide automation bindings", () => {
    render(<Harness onRun={() => {}} />);
    expect(screen.queryByTestId("loop-binding-badge")).not.toBeInTheDocument();
  });

  it("Should render exactly the server-filtered entries it receives", () => {
    render(<Harness entries={[reviewCatalogEntry]} onRun={() => {}} />);
    expect(screen.queryByTestId("loop-group-workspace")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-group-read-only")).toBeInTheDocument();
    expect(screen.getByText("review-and-fix")).toBeInTheDocument();
    expect(screen.queryByText("implement-tasks")).not.toBeInTheDocument();
  });

  it("Should offer clear filters when a server-filtered page is empty", () => {
    const onClearFilters = vi.fn();
    render(
      <Harness entries={[]} hasActiveFilters onClearFilters={onClearFilters} onRun={() => {}} />
    );
    expect(screen.getByTestId("loop-catalog-empty")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("loop-catalog-clear-filters"));
    expect(onClearFilters).toHaveBeenCalledTimes(1);
  });

  it("Should render the cards grid inside the same groups when view is cards", () => {
    render(<Harness onRun={() => {}} view="cards" />);
    expect(screen.getAllByTestId("loop-catalog-card-grid")).toHaveLength(1);
    expect(screen.getByTestId("loop-catalog-card-implement-tasks")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-group-workspace")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-catalog-row")).not.toBeInTheDocument();
  });

  it("Should launch a run from the card without navigating to the detail link", () => {
    const onRun = vi.fn();
    render(<Harness onRun={onRun} view="cards" />);
    const card = screen.getByTestId("loop-catalog-card-implement-tasks");
    const link = within(card).getByRole("link", { name: "Open implement-tasks" });
    const runButton = within(card).getByTestId("loop-catalog-run-implement-tasks");
    expect(link).not.toContainElement(runButton);
    fireEvent.click(runButton);
    expect(onRun).toHaveBeenCalledTimes(1);
    expect(onRun.mock.calls[0][0].name).toBe("implement-tasks");
  });

  it("Should launch a run inline without navigating to the detail row", () => {
    const onRun = vi.fn();
    render(<Harness onRun={onRun} />);
    const deliveryRow = screen
      .getByText("implement-tasks")
      .closest("[data-testid='loop-catalog-row']");
    const runButton = within(deliveryRow as HTMLElement).getByTestId(
      "loop-catalog-run-implement-tasks"
    );
    fireEvent.click(runButton);
    expect(onRun).toHaveBeenCalledTimes(1);
    expect(onRun.mock.calls[0][0].name).toBe("implement-tasks");
  });

  it("Should keep the inline Run button outside the detail link", () => {
    render(<Harness onRun={() => {}} />);
    const deliveryRow = screen
      .getByText("implement-tasks")
      .closest("[data-testid='loop-catalog-row']");
    const row = deliveryRow as HTMLElement;
    const link = within(row).getByRole("link", { name: "Open implement-tasks" });
    const runButton = within(row).getByTestId("loop-catalog-run-implement-tasks");
    expect(link).not.toContainElement(runButton);
  });

  it("Should expose a loading-aware control for the next server page", () => {
    const onLoadMore = vi.fn();
    const { rerender } = render(<Harness hasNextPage onLoadMore={onLoadMore} onRun={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "Load more loops" }));
    expect(onLoadMore).toHaveBeenCalledOnce();

    rerender(<Harness hasNextPage isFetchingNextPage onLoadMore={onLoadMore} onRun={() => {}} />);
    expect(screen.getByTestId("loop-catalog-load-more")).toBeDisabled();
    expect(screen.getByTestId("loop-catalog-load-more")).toHaveAttribute("aria-busy", "true");
  });

  it("Should state category, cap, and last-run recency as plain facts in both views", () => {
    const { rerender } = render(<Harness onRun={() => {}} />);
    const row = screen.getAllByTestId("loop-catalog-row")[0];
    expect(within(row).getByText("Engineering")).toBeInTheDocument();
    expect(within(row).getByText("3 inputs")).toBeInTheDocument();
    expect(within(row).getByText("iteration cap 50")).toBeInTheDocument();
    expect(within(row).getByText("looprun_running")).toBeInTheDocument();
    expect(row.querySelector('time[datetime="2026-07-05T12:00:00Z"]')).not.toBeNull();
    expect(within(row).queryByText("Built-in")).not.toBeInTheDocument();
    expect(within(row).queryByText("∞ cap")).not.toBeInTheDocument();

    rerender(<Harness onRun={() => {}} view="cards" />);
    const card = screen.getByTestId("loop-catalog-card-implement-tasks");
    expect(within(card).getByText("Engineering")).toBeInTheDocument();
    expect(within(card).getByText("3 inputs")).toBeInTheDocument();
    expect(within(card).getByText("iteration cap 50")).toBeInTheDocument();
    expect(within(card).getByText("looprun_running")).toBeInTheDocument();
    expect(card.querySelector('time[datetime="2026-07-05T12:00:00Z"]')).not.toBeNull();
  });

  it("Should demote best to a plain fact instead of a success-toned pill", () => {
    const withBest: LoopCatalogEntry = {
      ...loopCatalogFixtures[0],
      last_run: { ...loopCatalogFixtures[0].last_run!, best_generation: 2, best_score: 0.92 },
    };
    render(<Harness entries={[withBest]} onRun={() => {}} />);
    const row = screen.getByTestId("loop-catalog-row");
    expect(within(row).getByText("best Gen 2 · 0.92")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-catalog-best")).not.toBeInTheDocument();
  });

  it("Should use server kind facets in the group header and still drop empty groups", () => {
    render(
      <Harness
        facets={{ categories: {}, kinds: { read_only: 80, workspace: 5 }, statuses: {} }}
        onRun={() => {}}
      />
    );
    expect(within(screen.getByTestId("loop-group-read-only")).getByText("80")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-group-workspace")).not.toBeInTheDocument();
  });

  it("Should keep the roster empty inside the catalog when there are no loops", () => {
    render(<Harness entries={[]} onRun={() => {}} />);
    expect(screen.getByTestId("loop-catalog-empty")).toBeInTheDocument();
    expect(screen.getByText("No loops yet")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-catalog-clear-filters")).not.toBeInTheDocument();
  });
});
