import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
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

  it("Should fold a collapsed search to a toggle that opens the field and folds back when empty", async () => {
    const user = userEvent.setup();
    function CollapsedSearch() {
      const [value, setValue] = useState("");
      return (
        <ListingToolbar>
          <ListingToolbar.Leading>
            <ListingToolbar.Search
              aria-label="Search tasks"
              collapsed
              onChange={setValue}
              value={value}
            />
          </ListingToolbar.Leading>
        </ListingToolbar>
      );
    }
    render(<CollapsedSearch />);

    // No clipped field: just the named toggle.
    expect(screen.queryByRole("searchbox")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Search tasks" }));
    const field = screen.getByRole("searchbox", { name: "Search tasks" });
    expect(field).toHaveFocus();

    // Empty + Escape folds back, hands focus to the toggle and consumes the
    // key, so a host's document-level Escape handler leaves focus there.
    const escapes: boolean[] = [];
    const recordEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") escapes.push(event.defaultPrevented);
    };
    document.addEventListener("keydown", recordEscape);
    await user.keyboard("{Escape}");
    document.removeEventListener("keydown", recordEscape);
    expect(escapes).toEqual([true]);
    expect(screen.queryByRole("searchbox")).toBeNull();
    expect(screen.getByRole("button", { name: "Search tasks" })).toHaveFocus();

    // A field holding a query stays open after it loses focus.
    await user.click(screen.getByRole("button", { name: "Search tasks" }));
    await user.type(screen.getByRole("searchbox"), "deploy");
    await user.tab();
    expect(screen.getByRole("searchbox")).toHaveValue("deploy");
  });

  function StripSearch(props: { collapsed: boolean; disabled?: boolean; value?: string }) {
    return (
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search aria-label="Search tasks" {...props} />
          <button type="button">Filter</button>
        </ListingToolbar.Leading>
      </ListingToolbar>
    );
  }

  it("Should keep a focused empty field mounted when the strip narrows, folding only once it lets focus go", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<StripSearch collapsed={false} value="" />);
    await user.click(screen.getByRole("searchbox", { name: "Search tasks" }));

    rerender(<StripSearch collapsed value="" />);
    expect(screen.getByRole("searchbox", { name: "Search tasks" })).toHaveFocus();

    // Focus moves on: the empty field folds, and the toggle does not steal focus.
    await user.tab();
    expect(screen.getByRole("button", { name: "Filter" })).toHaveFocus();
    expect(screen.queryByRole("searchbox")).toBeNull();
    expect(screen.getByRole("button", { name: "Search tasks" })).toBeInTheDocument();
  });

  it("Should keep an uncontrolled query across strip width changes and fold once it is cleared", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<StripSearch collapsed={false} />);
    await user.type(screen.getByRole("searchbox"), "retained-query");
    await user.tab();

    // The live value, not the initial default, decides: a query never folds.
    rerender(<StripSearch collapsed />);
    expect(screen.getByRole("searchbox")).toHaveValue("retained-query");
    rerender(<StripSearch collapsed={false} />);
    expect(screen.getByRole("searchbox")).toHaveValue("retained-query");

    // Cleared and left, the narrow field folds; reopening starts from the live (empty) value.
    rerender(<StripSearch collapsed />);
    await user.click(screen.getByRole("button", { name: "Clear search field" }));
    await user.tab();
    expect(screen.queryByRole("searchbox")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Search tasks" }));
    expect(screen.getByRole("searchbox")).toHaveValue("");
    expect(screen.getByRole("searchbox")).toHaveFocus();
  });

  it("Should fold a disabled search to a disabled toggle that cannot open or take focus", async () => {
    const user = userEvent.setup();
    render(<StripSearch collapsed disabled value="" />);
    const toggle = screen.getByRole("button", { name: "Search tasks" });
    expect(toggle).toBeDisabled();

    await user.click(toggle);
    expect(screen.queryByRole("searchbox")).toBeNull();
    await user.tab();
    expect(toggle).not.toHaveFocus();
    expect(screen.getByRole("button", { name: "Filter" })).toHaveFocus();
  });
});
