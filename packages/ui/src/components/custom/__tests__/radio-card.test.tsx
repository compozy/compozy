import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RadioCard } from "../radio-card";

describe("RadioCard", () => {
  it("Should toggle selection on click and on keyboard activation", () => {
    const onSelect = vi.fn();
    render(<RadioCard selected={false} onSelect={onSelect} title="Sandbox template" />);
    const card = screen.getByRole("radio", { name: /sandbox template/i });
    fireEvent.click(card);
    expect(onSelect).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(card, { key: " " });
    expect(onSelect).toHaveBeenCalledTimes(2);
    fireEvent.keyDown(card, { key: "Enter" });
    expect(onSelect).toHaveBeenCalledTimes(3);
  });

  it("Should expose the selected state via aria-checked", () => {
    render(<RadioCard selected onSelect={() => undefined} title="Selected card" />);
    expect(screen.getByRole("radio", { name: /selected card/i })).toHaveAttribute(
      "aria-checked",
      "true"
    );
  });
});
