import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ScrollArea, ScrollBar } from "../scroll-area";

describe("ScrollArea", () => {
  it("Should render the root and viewport wrapper with stable data-slots", () => {
    const { container } = render(
      <ScrollArea className="size-24">
        <div className="h-96">long content</div>
      </ScrollArea>
    );
    expect(container.querySelector("[data-slot=scroll-area]")).toBeInTheDocument();
    expect(container.querySelector("[data-slot=scroll-area-viewport]")).toBeInTheDocument();
  });

  it("Should render a custom track/thumb when a scrollbar is kept mounted", () => {
    const { container } = render(
      <ScrollArea className="size-24">
        <ul>
          {Array.from({ length: 30 }, (_, i) => (
            <li key={i}>Item {i}</li>
          ))}
        </ul>
        <ScrollBar orientation="vertical" keepMounted />
      </ScrollArea>
    );
    const scrollbars = container.querySelectorAll("[data-slot=scroll-area-scrollbar]");
    const vertical = Array.from(scrollbars).find(
      el => el.getAttribute("data-orientation") === "vertical"
    );
    expect(vertical).toBeDefined();
    const thumb = vertical?.querySelector("[data-slot=scroll-area-thumb]");
    expect(thumb).not.toBeNull();
    expect(thumb).toHaveClass("rounded-full");
  });

  it("Should render a horizontal scrollbar when orientation='horizontal' is kept mounted", () => {
    const { container } = render(
      <ScrollArea className="w-24">
        <div className="flex w-96 gap-2">content</div>
        <ScrollBar orientation="horizontal" keepMounted />
      </ScrollArea>
    );
    const scrollbars = container.querySelectorAll("[data-slot=scroll-area-scrollbar]");
    const horizontal = Array.from(scrollbars).find(
      el => el.getAttribute("data-orientation") === "horizontal"
    );
    expect(horizontal).toBeDefined();
  });
});
