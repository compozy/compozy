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
type LoopCatalogEntry = import("../../types").LoopCatalogEntry;
type ListingViewMode = import("@agh/ui").ListingViewMode;

function Harness({
  onRun,
  onClearFilters = () => {},
  entries = loopCatalogFixtures,
  hasActiveFilters = false,
  hasNextPage = false,
  isFetchingNextPage = false,
  onLoadMore,
  view = "rows",
}: {
  onRun: (entry: LoopCatalogEntry) => void;
  onClearFilters?: () => void;
  entries?: readonly LoopCatalogEntry[];
  hasActiveFilters?: boolean;
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  onLoadMore?: () => void;
  view?: ListingViewMode;
}) {
  return (
    <LoopCatalog
      entries={entries}
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
  it("Should render grouped rows with success rate and the last-outcome pill", () => {
    render(<Harness onRun={() => {}} />);
    expect(screen.getByTestId("loop-group-read-only")).toBeInTheDocument();
    expect(screen.getByTestId("loop-group-workspace")).toBeInTheDocument();
    expect(screen.getByText("90%")).toBeInTheDocument();
    expect(screen.getByText("100%")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("Watching")).toBeInTheDocument();
  });

  it("Should not claim sampled catalog-wide automation bindings", () => {
    render(<Harness onRun={() => {}} />);
    expect(screen.queryByTestId("loop-binding-badge")).not.toBeInTheDocument();
  });

  it("Should render exactly the server-filtered entries it receives", () => {
    render(<Harness entries={[loopCatalogFixtures[1]]} onRun={() => {}} />);
    expect(screen.queryByTestId("loop-group-workspace")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-group-read-only")).toBeInTheDocument();
    expect(screen.getByText("reviews-watch")).toBeInTheDocument();
    expect(screen.queryByText("software-delivery")).not.toBeInTheDocument();
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

  it("Should render the cards grid when view is cards", () => {
    render(<Harness onRun={() => {}} view="cards" />);
    expect(screen.getByTestId("loop-catalog-card-grid")).toBeInTheDocument();
    expect(screen.getByTestId("loop-catalog-card-software-delivery")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-catalog")).not.toBeInTheDocument();
  });

  it("Should launch a run from the card without navigating to the detail link", () => {
    const onRun = vi.fn();
    render(<Harness onRun={onRun} view="cards" />);
    const card = screen.getByTestId("loop-catalog-card-software-delivery");
    const link = within(card).getByRole("link", { name: "Open software-delivery" });
    const runButton = within(card).getByTestId("loop-catalog-run-software-delivery");
    expect(link).not.toContainElement(runButton);
    fireEvent.click(runButton);
    expect(onRun).toHaveBeenCalledTimes(1);
    expect(onRun.mock.calls[0][0].name).toBe("software-delivery");
  });

  it("Should launch a run inline without navigating to the detail row", () => {
    const onRun = vi.fn();
    render(<Harness onRun={onRun} />);
    const deliveryRow = screen
      .getByText("software-delivery")
      .closest("[data-testid='loop-catalog-row']");
    const runButton = within(deliveryRow as HTMLElement).getByTestId(
      "loop-catalog-run-software-delivery"
    );
    fireEvent.click(runButton);
    expect(onRun).toHaveBeenCalledTimes(1);
    expect(onRun.mock.calls[0][0].name).toBe("software-delivery");
  });

  it("Should keep the inline Run button outside the detail link", () => {
    render(<Harness onRun={() => {}} />);
    const deliveryRow = screen
      .getByText("software-delivery")
      .closest("[data-testid='loop-catalog-row']");
    const row = deliveryRow as HTMLElement;
    const link = within(row).getByRole("link", { name: "Open software-delivery" });
    const runButton = within(row).getByTestId("loop-catalog-run-software-delivery");
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
});
