import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { RadioCardGroup, type RadioCardGroupOption } from "../radio-card-group";

type Level = "ask" | "read" | "edit" | "all";

const OPTIONS: RadioCardGroupOption<Level>[] = [
  { value: "ask", title: "Ask first" },
  { value: "read", title: "Allow reading" },
  { value: "edit", title: "Edit", disabled: true },
  { value: "all", title: "Allow everything" },
];

function Harness({ initial = "ask" as Level | null, disabled = false }) {
  const [value, setValue] = useState<Level | null>(initial);
  return (
    <RadioCardGroup
      aria-label="Level"
      disabled={disabled}
      onValueChange={setValue}
      options={OPTIONS}
      value={value}
    />
  );
}

describe("RadioCardGroup", () => {
  it("Should keep one tab stop on the selected card", () => {
    render(<Harness initial="read" />);
    const tabbable = screen.getAllByRole("radio").filter(radio => radio.tabIndex === 0);
    expect(tabbable).toHaveLength(1);
    expect(tabbable[0]).toHaveAccessibleName("Allow reading");
  });

  it("Should give the first enabled card the tab stop when nothing is chosen", () => {
    render(<Harness initial={null} />);
    expect(screen.getByRole("radio", { name: "Ask first" })).toHaveAttribute("tabindex", "0");
  });

  it("Should move and select with arrows, skipping disabled cards and wrapping", () => {
    render(<Harness />);
    const ask = screen.getByRole("radio", { name: "Ask first" });
    fireEvent.keyDown(ask, { key: "ArrowDown" });
    const read = screen.getByRole("radio", { name: "Allow reading" });
    expect(read).toHaveAttribute("aria-checked", "true");
    expect(read).toHaveFocus();
    fireEvent.keyDown(read, { key: "ArrowRight" });
    const all = screen.getByRole("radio", { name: "Allow everything" });
    expect(all).toHaveAttribute("aria-checked", "true");
    fireEvent.keyDown(all, { key: "ArrowDown" });
    expect(screen.getByRole("radio", { name: "Ask first" })).toHaveAttribute(
      "aria-checked",
      "true"
    );
    fireEvent.keyDown(screen.getByRole("radio", { name: "Ask first" }), { key: "ArrowUp" });
    expect(all).toHaveAttribute("aria-checked", "true");
  });

  it("Should jump to the ends with Home and End", () => {
    render(<Harness initial="read" />);
    fireEvent.keyDown(screen.getByRole("radio", { name: "Allow reading" }), { key: "End" });
    expect(screen.getByRole("radio", { name: "Allow everything" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("radio", { name: "Allow everything" }), { key: "Home" });
    expect(screen.getByRole("radio", { name: "Ask first" })).toHaveAttribute(
      "aria-checked",
      "true"
    );
  });

  it("Should disable every card when the group is disabled", () => {
    render(<Harness disabled />);
    for (const radio of screen.getAllByRole("radio")) expect(radio).toBeDisabled();
  });
});
