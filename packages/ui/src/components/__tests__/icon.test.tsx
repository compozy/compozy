import { render } from "@testing-library/react";
import { Sparkles } from "lucide-react";
import { describe, expect, it } from "vitest";

import { Icon } from "../icon";

describe("Icon", () => {
  it("Should render at 14 px with stroke 1.75 by default", () => {
    const { container } = render(<Icon as={Sparkles} />);
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("width")).toBe("14");
    expect(svg?.getAttribute("height")).toBe("14");
    expect(svg?.getAttribute("stroke-width")).toBe("1.75");
    expect(svg?.getAttribute("data-icon-size")).toBe("default");
  });

  it("Should render at 11 px with stroke 2 when size is xs", () => {
    const { container } = render(<Icon as={Sparkles} size="xs" />);
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("width")).toBe("11");
    expect(svg?.getAttribute("height")).toBe("11");
    expect(svg?.getAttribute("stroke-width")).toBe("2");
  });

  it("Should map sm to 12 px and lg to 16 px (stroke 1.75)", () => {
    const { container: sm } = render(<Icon as={Sparkles} size="sm" />);
    expect(sm.querySelector("svg")?.getAttribute("width")).toBe("12");
    expect(sm.querySelector("svg")?.getAttribute("stroke-width")).toBe("1.75");

    const { container: lg } = render(<Icon as={Sparkles} size="lg" />);
    expect(lg.querySelector("svg")?.getAttribute("width")).toBe("16");
    expect(lg.querySelector("svg")?.getAttribute("stroke-width")).toBe("1.75");
  });

  it("Should accept an explicit strokeWidth override", () => {
    const { container } = render(<Icon as={Sparkles} strokeWidth={1} />);
    expect(container.querySelector("svg")?.getAttribute("stroke-width")).toBe("1");
  });

  it("Should merge consumer className onto the rendered svg", () => {
    const { container } = render(<Icon as={Sparkles} className="text-accent" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal).toContain("text-accent");
  });
});
