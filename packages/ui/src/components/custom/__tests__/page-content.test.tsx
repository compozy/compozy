import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PAGE_CONTENT_GUTTER, PageContent } from "../page-content";

describe("PageContent", () => {
  it("Should apply the shared gutter and default listing density", () => {
    render(
      <PageContent data-testid="content">
        <p>Body</p>
      </PageContent>
    );

    const content = screen.getByTestId("content");
    expect(content).toHaveAttribute("data-slot", "page-content");
    expect(content).toHaveAttribute("data-density", "listing");
    expect(content.className).toContain(PAGE_CONTENT_GUTTER.split(" ")[0]!);
    expect(content.className).toContain("px-9");
    expect(content.className).toContain("max-w-content-max");
    expect(content.className).toContain("min-h-full");
    expect(screen.getByText("Body")).toBeInTheDocument();
  });

  it("Should emit density=comfortable when set", () => {
    render(
      <PageContent density="comfortable" data-testid="content">
        body
      </PageContent>
    );
    expect(screen.getByTestId("content")).toHaveAttribute("data-density", "comfortable");
  });
});
