import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "../button";

describe("Button", () => {
  it('Should render a `data-slot="button"` root', () => {
    render(<Button>Action</Button>);
    const button = screen.getByRole("button", { name: /action/i });
    expect(button).toHaveAttribute("data-slot", "button");
  });

  it("Should mark itself as disabled when the disabled prop is set", () => {
    render(<Button disabled>D</Button>);
    expect(screen.getByRole("button", { name: /d/i })).toBeDisabled();
  });

  it("Should forward className alongside variant defaults", () => {
    render(<Button className="custom-tail">F</Button>);
    expect(screen.getByRole("button", { name: /f/i }).className).toContain("custom-tail");
  });

  it("Should apply solid danger fill and readable ink on destructive-solid", () => {
    render(<Button variant="destructive-solid">Kill</Button>);
    const button = screen.getByRole("button", { name: /kill/i });
    expect(button.className).toContain("bg-danger");
    expect(button.className).toContain("text-accent-ink");
    expect(button.className).not.toContain("bg-danger-tint");
  });

  it("Should render a trailing kbd hint that stays out of the accessible name", () => {
    render(
      <Button variant="primary" kbd="↵">
        Allow once
      </Button>
    );
    const button = screen.getByRole("button", { name: "Allow once" });
    const hint = button.querySelector('[data-slot="kbd"]');
    expect(hint).toHaveTextContent("↵");
    expect(hint).toHaveAttribute("aria-hidden", "true");
    expect(button.lastElementChild).toBe(hint);
  });

  it("Should render children untouched when no kbd hint is given", () => {
    render(<Button>Plain</Button>);
    expect(
      screen.getByRole("button", { name: "Plain" }).querySelector('[data-slot="kbd"]')
    ).toBeNull();
  });
});
