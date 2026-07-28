import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Checkbox } from "../checkbox";

describe("Checkbox", () => {
  it("Should render with the data-slot attribute", () => {
    const { container } = render(<Checkbox aria-label="accept" />);
    expect(container.querySelector("[data-slot=checkbox]")).not.toBeNull();
  });

  it("Should toggle between unchecked and checked when clicked", async () => {
    const user = userEvent.setup();
    render(<Checkbox aria-label="accept" />);
    const box = screen.getByRole("checkbox", { name: "accept" });
    expect(box).toHaveAttribute("aria-checked", "false");
    await user.click(box);
    expect(box).toHaveAttribute("aria-checked", "true");
  });

  it("Should fire onCheckedChange with the new value", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(<Checkbox aria-label="accept" onCheckedChange={handleChange} />);
    await user.click(screen.getByRole("checkbox", { name: "accept" }));
    expect(handleChange.mock.calls[0]?.[0]).toBe(true);
  });

  it("Should not respond to clicks when disabled", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(<Checkbox aria-label="locked" disabled onCheckedChange={handleChange} />);
    await user.click(screen.getByRole("checkbox", { name: "locked" }));
    expect(handleChange).not.toHaveBeenCalled();
  });
});
