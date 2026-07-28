import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ListingToolbar } from "../listing-toolbar";

describe("ListingToolbar", () => {
  it("Should compose Leading search/filters and Trailing view toggle", async () => {
    const onSearchChange = vi.fn();
    const onViewChange = vi.fn();
    const user = userEvent.setup();

    render(
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search onChange={onSearchChange} placeholder="Search skills" value="" />
          <ListingToolbar.Filters>
            <span data-testid="filters-slot">filters</span>
          </ListingToolbar.Filters>
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <ListingToolbar.ViewToggle onChange={onViewChange} value="rows" />
        </ListingToolbar.Trailing>
      </ListingToolbar>
    );

    expect(screen.getByTestId("listing-toolbar")).toHaveAttribute("data-slot", "listing-toolbar");
    expect(screen.getByTestId("listing-search-input")).toBeInTheDocument();
    expect(screen.getByTestId("filters-slot")).toBeInTheDocument();
    expect(screen.getByTestId("listing-view-toggle")).toBeInTheDocument();

    await user.type(screen.getByRole("searchbox"), "alpha");
    expect(onSearchChange).toHaveBeenCalled();

    await user.click(screen.getByTestId("listing-view-cards"));
    expect(onViewChange).toHaveBeenCalledWith("cards");
  });

  it("Should allow omitting Trailing when cards-only surfaces do not need a toggle", () => {
    render(
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search onChange={vi.fn()} value="" />
        </ListingToolbar.Leading>
      </ListingToolbar>
    );

    expect(screen.queryByTestId("listing-view-toggle")).not.toBeInTheDocument();
  });
});
