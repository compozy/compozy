import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProseH2, ProseH3 } from "../prose";

describe("blog prose headings", () => {
  it("derives stable anchors when compiled blog headings do not pass an id", () => {
    render(
      <>
        <ProseH2>Why agents need a workplace</ProseH2>
        <ProseH3>
          Tools and <code>MCP</code>
        </ProseH3>
      </>
    );

    expect(screen.getByRole("heading", { name: "Why agents need a workplace" }).id).toBe(
      "why-agents-need-a-workplace"
    );
    expect(screen.getByRole("heading", { name: "Tools and MCP" }).id).toBe("tools-and-mcp");
  });

  it("preserves explicit heading ids from MDX when present", () => {
    render(<ProseH2 id="custom-anchor">Custom Anchor</ProseH2>);

    expect(screen.getByRole("heading", { name: "Custom Anchor" }).id).toBe("custom-anchor");
  });
});
