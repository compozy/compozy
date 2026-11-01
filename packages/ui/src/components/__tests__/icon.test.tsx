import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Sparkles } from "../../icons";
import { Icon } from "../icon";

describe("Icon", () => {
  it("Should accept an explicit strokeWidth override", () => {
    const { container } = render(<Icon as={Sparkles} strokeWidth={1} />);
    expect(container.querySelector("svg")?.getAttribute("stroke-width")).toBe("1");
  });

  it("Should merge consumer className onto the rendered svg", () => {
    const { container } = render(<Icon as={Sparkles} className="text-brand" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal).toContain("text-brand");
  });
});
