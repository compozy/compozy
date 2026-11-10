import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProseH2, ProseH3, ProseTable } from "../prose";

describe("blog prose", () => {
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

  it("puts tables in a keyboard-reachable region that scrolls on its own", () => {
    render(
      <ProseTable>
        <tbody>
          <tr>
            <td>cell</td>
          </tr>
        </tbody>
      </ProseTable>
    );

    const region = screen.getByRole("region", { name: "Table" });
    expect(region.getAttribute("tabindex")).toBe("0");
    expect(region.querySelector("table")).not.toBeNull();
  });
});
