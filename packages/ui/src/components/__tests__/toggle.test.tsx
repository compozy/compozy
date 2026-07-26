import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { Toggle } from "../toggle";

describe("Toggle", () => {
  it("Should render unpressed by default", () => {
    render(<Toggle aria-label="bold">B</Toggle>);
    const toggle = screen.getByRole("button", { name: "bold" });
    expect(toggle).toHaveAttribute("aria-pressed", "false");
  });

  it("Should flip aria-pressed when clicked", async () => {
    const user = userEvent.setup();
    render(<Toggle aria-label="bold">B</Toggle>);
    const toggle = screen.getByRole("button", { name: "bold" });
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-pressed", "true");
  });

  it("Should respect defaultPressed", () => {
    render(
      <Toggle aria-label="italic" defaultPressed>
        I
      </Toggle>
    );
    expect(screen.getByRole("button", { name: "italic" })).toHaveAttribute("aria-pressed", "true");
  });
});
