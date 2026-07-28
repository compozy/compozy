import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { ToggleGroup, ToggleGroupItem } from "../toggle-group";

describe("ToggleGroup", () => {
  it("Should render all items with the toggle-group-item slot", () => {
    const { container } = render(
      <ToggleGroup>
        <ToggleGroupItem value="left" aria-label="Align left">
          L
        </ToggleGroupItem>
        <ToggleGroupItem value="right" aria-label="Align right">
          R
        </ToggleGroupItem>
      </ToggleGroup>
    );
    const items = container.querySelectorAll("[data-slot=toggle-group-item]");
    expect(items.length).toBe(2);
  });

  it("Should enforce single-selection by default", async () => {
    const user = userEvent.setup();
    render(
      <ToggleGroup defaultValue={["left"]}>
        <ToggleGroupItem value="left" aria-label="Align left">
          L
        </ToggleGroupItem>
        <ToggleGroupItem value="right" aria-label="Align right">
          R
        </ToggleGroupItem>
      </ToggleGroup>
    );
    const left = screen.getByRole("button", { name: "Align left" });
    const right = screen.getByRole("button", { name: "Align right" });
    expect(left).toHaveAttribute("aria-pressed", "true");
    await user.click(right);
    expect(right).toHaveAttribute("aria-pressed", "true");
    expect(left).toHaveAttribute("aria-pressed", "false");
  });

  it("Should accumulate pressed items with multiple=true", async () => {
    const user = userEvent.setup();
    render(
      <ToggleGroup multiple defaultValue={["bold"]}>
        <ToggleGroupItem value="bold" aria-label="Bold">
          B
        </ToggleGroupItem>
        <ToggleGroupItem value="italic" aria-label="Italic">
          I
        </ToggleGroupItem>
      </ToggleGroup>
    );
    const bold = screen.getByRole("button", { name: "Bold" });
    const italic = screen.getByRole("button", { name: "Italic" });
    await user.click(italic);
    expect(bold).toHaveAttribute("aria-pressed", "true");
    expect(italic).toHaveAttribute("aria-pressed", "true");
  });

  it("Should mark the group vertical when orientation='vertical'", () => {
    const { container } = render(
      <ToggleGroup orientation="vertical">
        <ToggleGroupItem value="one" aria-label="one">
          1
        </ToggleGroupItem>
      </ToggleGroup>
    );
    const root = container.querySelector("[data-slot=toggle-group]") as HTMLElement | null;
    expect(root).toHaveAttribute("data-orientation", "vertical");
  });
});
