import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { MetadataTile } from "../metadata-tile";

describe("MetadataTile", () => {
  it("Should render the label and value", () => {
    const { container } = render(<MetadataTile label="Last run" value="3m ago" />);
    const label = container.querySelector<HTMLElement>('[data-slot="metadata-tile-label"]');
    expect(label?.textContent).toBe("Last run");
    expect(screen.getByText("3m ago")).toBeInTheDocument();
  });
});
