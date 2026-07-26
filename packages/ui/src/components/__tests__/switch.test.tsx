import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Switch } from "../switch";

describe("Switch", () => {
  it("Should render with the data-slot and default size attribute", () => {
    const { container } = render(<Switch aria-label="toggle streaming" />);
    const root = container.querySelector("[data-slot=switch]") as HTMLElement | null;
    expect(root).not.toBeNull();
    expect(root).toHaveAttribute("data-size", "default");
  });

  it("Should toggle between unchecked and checked when clicked", async () => {
    const user = userEvent.setup();
    render(<Switch aria-label="toggle streaming" />);
    const toggle = screen.getByRole("switch", { name: "toggle streaming" });
    expect(toggle).toHaveAttribute("aria-checked", "false");
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-checked", "true");
  });

  it("Should fire onCheckedChange with the new value", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(<Switch aria-label="toggle streaming" onCheckedChange={handleChange} />);
    await user.click(screen.getByRole("switch", { name: "toggle streaming" }));
    expect(handleChange).toHaveBeenCalledWith(true, expect.anything());
  });

  it("Should honor the sm size variant", () => {
    const { container } = render(<Switch aria-label="compact" size="sm" />);
    const root = container.querySelector("[data-slot=switch]") as HTMLElement | null;
    expect(root).toHaveAttribute("data-size", "sm");
  });

  it("Should not respond to clicks when disabled", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(<Switch aria-label="locked" disabled onCheckedChange={handleChange} />);
    await user.click(screen.getByRole("switch", { name: "locked" }));
    expect(handleChange).not.toHaveBeenCalled();
  });
});
