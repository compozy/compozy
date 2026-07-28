import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DirectionProvider, useDirection } from "../direction";

function DirReader() {
  const direction = useDirection();
  return <span data-testid="dir">{direction}</span>;
}

describe("DirectionProvider", () => {
  it("Should pass through an explicit ltr direction to descendants", () => {
    const { getByTestId } = render(
      <DirectionProvider direction="ltr">
        <DirReader />
      </DirectionProvider>
    );
    expect(getByTestId("dir").textContent).toBe("ltr");
  });

  it("Should forward rtl to descendants", () => {
    const { getByTestId } = render(
      <DirectionProvider direction="rtl">
        <DirReader />
      </DirectionProvider>
    );
    expect(getByTestId("dir").textContent).toBe("rtl");
  });
});
