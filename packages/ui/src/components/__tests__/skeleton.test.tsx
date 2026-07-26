import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Skeleton, SkeletonRows } from "../skeleton";

describe("Skeleton", () => {
  it("Should use animate-shimmer instead of animate-pulse", () => {
    const { container } = render(<Skeleton className="h-4 w-12" />);
    const node = container.querySelector('[data-slot="skeleton"]');
    expect(node?.className).toContain("animate-shimmer");
    expect(node?.className).not.toContain("animate-pulse");
  });
});

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
