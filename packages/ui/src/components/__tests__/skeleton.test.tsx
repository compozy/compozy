import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  CatalogCardSkeleton,
  ItemSkeleton,
  ListingRowSkeleton,
  PropertyRowSkeleton,
  TableRowSkeleton,
} from "../custom/content-skeleton";
import { Skeleton, SkeletonRegion, SkeletonRows, SkeletonText } from "../skeleton";
import { Table, TableBody } from "../table";

function slots(container: HTMLElement, slot: string): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(`[data-slot="${slot}"]`));
}

describe("SkeletonRows", () => {
  it("Should render the requested number of rows", () => {
    const { container } = render(<SkeletonRows count={4} />);
    expect(container.querySelectorAll('[data-slot="skeleton-row"]')).toHaveLength(4);
  });

  it("Should repeat custom row content for each row", () => {
    const { container } = render(
      <SkeletonRows count={2}>
        <span data-testid="custom-line" />
      </SkeletonRows>
    );
    expect(container.querySelectorAll('[data-testid="custom-line"]')).toHaveLength(2);
  });
});

describe("SkeletonRegion", () => {
  it("Should announce the loading area once, as a busy status named by hidden text", () => {
    render(
      <SkeletonRegion label="Loading providers">
        <Skeleton data-testid="block" />
      </SkeletonRegion>
    );

    const region = screen.getByRole("status");
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(region).toHaveTextContent("Loading providers");
    // The label leads, so `:last-child` selectors on the placeholder rows still hold.
    expect(region.lastElementChild).toBe(screen.getByTestId("block"));
  });

  it("Should fall back to a generic label", () => {
    render(<SkeletonRegion />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading");
  });
});

describe("SkeletonText", () => {
  it("Should stand in for one line and stay out of the accessibility tree", () => {
    const { container } = render(<SkeletonText />);
    expect(slots(container, "skeleton-text-line")).toHaveLength(1);
    expect(slots(container, "skeleton-text")[0]).toHaveAttribute("aria-hidden", "true");
  });

  it("Should narrow only the last line of a multi-line block", () => {
    const { container } = render(<SkeletonText lines={3} lastLineWidth={0.4} />);

    const bars = slots(container, "skeleton");
    expect(bars).toHaveLength(3);
    expect(bars.map(bar => bar.style.width)).toEqual(["", "", "40%"]);
  });

  it("Should keep a single line at the caller's width", () => {
    const { container } = render(<SkeletonText lastLineWidth={0.4} />);
    expect(slots(container, "skeleton")[0].style.width).toBe("");
  });
});

describe("ListingRowSkeleton", () => {
  it("Should render the requested count of inert listing rows hidden from assistive tech", () => {
    const { container } = render(<ListingRowSkeleton count={4} />);

    const rows = slots(container, "listing-row");
    expect(rows).toHaveLength(4);
    for (const row of rows) {
      expect(row).toHaveAttribute("aria-hidden", "true");
      expect(row).not.toHaveAttribute("data-interactive");
      expect(row.querySelector("[data-slot='listing-row-icon']")).not.toBeNull();
      expect(row.querySelector("[data-slot='listing-row-title']")).not.toBeNull();
    }
    expect(container.querySelector("a, button")).toBeNull();
  });

  it("Should add the optional lines and trail only when asked", () => {
    const { container, rerender } = render(<ListingRowSkeleton count={2} />);
    expect(slots(container, "listing-row-description")).toHaveLength(0);
    expect(slots(container, "listing-row-meta")).toHaveLength(2);
    expect(slots(container, "listing-row-stat")).toHaveLength(2);

    rerender(<ListingRowSkeleton count={2} description meta={false} trail="pill" />);
    expect(slots(container, "listing-row-description")).toHaveLength(2);
    expect(slots(container, "listing-row-meta")).toHaveLength(0);
    expect(slots(container, "listing-row-stat")).toHaveLength(0);
    expect(slots(container, "listing-row-trail")).toHaveLength(2);

    rerender(<ListingRowSkeleton count={2} trail="none" />);
    expect(slots(container, "listing-row-trail")).toHaveLength(0);
  });
});

describe("ItemSkeleton", () => {
  it("Should render real items with the real size and variant", () => {
    const { container } = render(<ItemSkeleton count={3} size="xs" variant="outline" />);

    const items = slots(container, "item");
    expect(items).toHaveLength(3);
    for (const item of items) {
      expect(item).toHaveAttribute("aria-hidden", "true");
      expect(item).toHaveAttribute("data-size", "xs");
      expect(item).toHaveAttribute("data-variant", "outline");
    }
  });

  it("Should compose media, description and actions from the toggles", () => {
    const { container, rerender } = render(<ItemSkeleton count={1} />);
    expect(slots(container, "item-media")[0]).toHaveAttribute("data-variant", "icon");
    expect(slots(container, "item-description")).toHaveLength(1);
    expect(slots(container, "item-actions")).toHaveLength(0);

    rerender(<ItemSkeleton count={1} media="none" description={false} actions />);
    expect(slots(container, "item-media")).toHaveLength(0);
    expect(slots(container, "item-description")).toHaveLength(0);
    expect(slots(container, "item-actions")).toHaveLength(1);
  });
});

describe("CatalogCardSkeleton", () => {
  it("Should render the standard card stack inside the real card surface", () => {
    const { container } = render(<CatalogCardSkeleton count={2} />);

    const cards = slots(container, "catalog-card");
    expect(cards).toHaveLength(2);
    for (const card of cards) {
      expect(card).toHaveAttribute("aria-hidden", "true");
      for (const slot of [
        "catalog-card-logo",
        "catalog-card-title",
        "catalog-card-meta",
        "catalog-card-description",
        "catalog-card-actions",
      ]) {
        expect(card.querySelector(`[data-slot="${slot}"]`)).not.toBeNull();
      }
    }
  });

  it("Should omit the parts that are switched off", () => {
    const { container } = render(<CatalogCardSkeleton descriptionLines={0} actions={false} />);
    expect(slots(container, "catalog-card-description")).toHaveLength(0);
    expect(slots(container, "catalog-card-actions")).toHaveLength(0);
  });

  it("Should keep the real card surface around custom children", () => {
    const { container } = render(
      <CatalogCardSkeleton className="grid">
        <span data-testid="custom" />
      </CatalogCardSkeleton>
    );

    const card = slots(container, "catalog-card")[0];
    expect(card).toContainElement(screen.getByTestId("custom"));
    expect(card).toHaveClass("grid");
    expect(slots(container, "catalog-card-logo")).toHaveLength(0);
  });
});

describe("TableRowSkeleton", () => {
  it("Should fill the real table body with rows of real cells", () => {
    const { container } = render(
      <Table>
        <TableBody>
          <TableRowSkeleton count={4} columns={3} />
        </TableBody>
      </Table>
    );

    const rows = slots(container, "table-row");
    expect(rows).toHaveLength(4);
    for (const row of rows) {
      expect(row).toHaveAttribute("aria-hidden", "true");
      expect(row.querySelectorAll('[data-slot="table-cell"]')).toHaveLength(3);
    }
  });

  it("Should take its column count from an explicit column list", () => {
    const { container } = render(
      <Table>
        <TableBody>
          <TableRowSkeleton count={1} columns={[{ className: "w-24" }, { align: "end" }]} />
        </TableBody>
      </Table>
    );
    expect(slots(container, "table-cell")).toHaveLength(2);
  });

  // Invariant: a column's cell carries the classes the real cell has, so a column the header hides
  // in a narrow pane is hidden in the placeholder rows too, and a column that brings its own
  // placeholder gets it in place of the default bar, told which row it is drawing.
  it("Should give each column's cell its own classes and let a column bring its placeholder", () => {
    const { container } = render(
      <Table>
        <TableBody>
          <TableRowSkeleton
            count={2}
            columns={[
              { cellClassName: "hidden @3xl:table-cell", className: "w-24" },
              {
                cellClassName: "w-14",
                placeholder: row => <span data-testid={`placeholder-${row}`} />,
              },
            ]}
          />
        </TableBody>
      </Table>
    );

    for (const row of slots(container, "table-row")) {
      const [first, second] = Array.from(row.querySelectorAll('[data-slot="table-cell"]'));
      expect(first).toHaveClass("hidden", "@3xl:table-cell");
      expect(first.querySelector('[data-slot="skeleton-text"]')).toHaveClass("w-24");
      expect(second).toHaveClass("w-14");
      expect(second.querySelector('[data-slot="skeleton-text"]')).toBeNull();
    }
    expect(screen.getByTestId("placeholder-0")).toBeInTheDocument();
    expect(screen.getByTestId("placeholder-1")).toBeInTheDocument();
  });
});

describe("PropertyRowSkeleton", () => {
  it("Should render real property rows with a placeholder label and value", () => {
    const { container } = render(<PropertyRowSkeleton count={3} variant="facts" />);

    const rows = slots(container, "property-row");
    expect(rows).toHaveLength(3);
    for (const row of rows) {
      expect(row).toHaveAttribute("aria-hidden", "true");
      expect(row).toHaveAttribute("data-variant", "facts");
      expect(
        row.querySelector('[data-slot="property-row-label"] [data-slot="skeleton"]')
      ).not.toBeNull();
      expect(
        row.querySelector('[data-slot="property-row-value"] [data-slot="skeleton"]')
      ).not.toBeNull();
    }
  });
});
