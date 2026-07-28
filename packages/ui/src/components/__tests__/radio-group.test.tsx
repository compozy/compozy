import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RadioGroup, RadioGroupItem } from "../radio-group";

describe("RadioGroup", () => {
  it("Should render the group and items with data-slots", () => {
    const { container } = render(
      <RadioGroup aria-label="mode">
        <RadioGroupItem aria-label="fast" value="fast" />
      </RadioGroup>
    );
    expect(container.querySelector("[data-slot=radio-group]")).not.toBeNull();
    expect(container.querySelector("[data-slot=radio-group-item]")).not.toBeNull();
  });

  it("Should select an item when clicked", async () => {
    const user = userEvent.setup();
    render(
      <RadioGroup aria-label="mode">
        <RadioGroupItem aria-label="fast" value="fast" />
        <RadioGroupItem aria-label="slow" value="slow" />
      </RadioGroup>
    );
    const fast = screen.getByRole("radio", { name: "fast" });
    expect(fast).toHaveAttribute("aria-checked", "false");
    await user.click(fast);
    expect(fast).toHaveAttribute("aria-checked", "true");
  });

  it("Should fire onValueChange with the selected value", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(
      <RadioGroup aria-label="mode" onValueChange={handleChange}>
        <RadioGroupItem aria-label="fast" value="fast" />
      </RadioGroup>
    );
    await user.click(screen.getByRole("radio", { name: "fast" }));
    expect(handleChange.mock.calls[0]?.[0]).toBe("fast");
  });

  it("Should not select when the group is disabled", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(
      <RadioGroup aria-label="mode" disabled onValueChange={handleChange}>
        <RadioGroupItem aria-label="fast" value="fast" />
      </RadioGroup>
    );
    await user.click(screen.getByRole("radio", { name: "fast" }));
    expect(handleChange).not.toHaveBeenCalled();
  });
});
