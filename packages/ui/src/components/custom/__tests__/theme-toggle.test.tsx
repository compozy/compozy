// Invariant: the toggle names and shows the theme it switches TO, and hands the press to its owner.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ThemeToggle } from "../theme-toggle";

describe("ThemeToggle", () => {
  it("Should offer light mode with a sun while dark is painted", () => {
    render(<ThemeToggle resolvedTheme="dark" />);
    const button = screen.getByRole("button", { name: "Switch to light mode" });
    expect(button).toHaveAttribute("type", "button");
    expect(button.querySelector("svg.lucide-sun")).not.toBeNull();
  });

  it("Should offer dark mode with a moon while light is painted", () => {
    render(<ThemeToggle resolvedTheme="light" />);
    const button = screen.getByRole("button", { name: "Switch to dark mode" });
    expect(button.querySelector("svg.lucide-moon")).not.toBeNull();
  });

  it("Should hand each press to the owner", async () => {
    const onClick = vi.fn();
    render(<ThemeToggle onClick={onClick} resolvedTheme="dark" />);
    await userEvent.click(screen.getByRole("button", { name: "Switch to light mode" }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
