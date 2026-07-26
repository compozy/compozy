import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TypingDots } from "../typing-dots";

describe("TypingDots", () => {
  it("Should render three animated dots with staggered delays", () => {
    const { container } = render(<TypingDots />);
    const root = container.querySelector<HTMLElement>('[data-slot="typing-dots"]');
    expect(root).not.toBeNull();
    expect(root?.getAttribute("aria-hidden")).toBe("true");

    const dots = root?.querySelectorAll("span");
    expect(dots?.length).toBe(3);
    expect(dots?.[0].className).toContain("typing-bounce");
    expect(dots?.[1].className).toContain("0.15s");
    expect(dots?.[2].className).toContain("0.3s");
  });
});
