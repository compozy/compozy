import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ListingPage } from "../listing-page";

describe("ListingPage", () => {
  it("Should compose the listing shell, banner, and body content", () => {
    render(
      <ListingPage data-testid="listing" banner={<p>Cached results</p>}>
        <p data-testid="listing-body">Listing body</p>
      </ListingPage>
    );

    expect(screen.getByTestId("listing")).toHaveAttribute("data-slot", "listing-page");
    const content = screen.getByTestId("listing").querySelector('[data-slot="page-content"]');
    expect(content).not.toBeNull();
    expect(content?.className).toContain("px-9");
    expect(content?.className).toContain("max-w-content-max");
    expect(content?.className).toContain("min-h-full");
    expect(screen.getByText("Cached results")).toBeInTheDocument();
    expect(screen.getByTestId("listing-body")).toHaveTextContent("Listing body");
  });
});
