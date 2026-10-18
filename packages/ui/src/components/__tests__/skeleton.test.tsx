import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SkeletonRows } from "../skeleton";

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
