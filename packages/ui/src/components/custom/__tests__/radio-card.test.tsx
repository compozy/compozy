import { fireEvent, render, screen } from "@testing-library/react";
import { Hourglass } from "lucide-react";
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

  it("Should keep the default icon slot at 20px and opt into the 28px well", () => {
    const { rerender } = render(
      <RadioCard icon={Hourglass} onSelect={() => undefined} selected title="Drain" />
    );
    const defaultWell = screen
      .getByRole("radio", { name: /drain/i })
      .querySelector('[data-slot="radio-card-icon-well"]');
    expect(defaultWell).toHaveAttribute("data-icon-well-size", "default");
    expect(defaultWell?.className).toContain("size-5");
    expect(defaultWell?.className).not.toContain("size-7");
    expect(defaultWell?.querySelector("svg")).toHaveClass("size-3");

    rerender(
      <RadioCard
        icon={Hourglass}
        iconWellSize="lg"
        onSelect={() => undefined}
        selected
        title="Drain"
      />
    );
    const lgWell = screen
      .getByRole("radio", { name: /drain/i })
      .querySelector('[data-slot="radio-card-icon-well"]');
    expect(lgWell).toHaveAttribute("data-icon-well-size", "lg");
    expect(lgWell?.className).toContain("size-7");
    expect(lgWell?.className).toContain("rounded-sm");
    expect(lgWell?.className).toContain("bg-badge-fill");
    expect(lgWell?.querySelector("svg")).toHaveClass("size-3.5");
  });

  it("Should keep the unselected lg well muted with an inset hairline", () => {
    render(
      <RadioCard
        icon={Hourglass}
        iconWellSize="lg"
        onSelect={() => undefined}
        selected={false}
        title="Drain"
      />
    );
    const well = screen
      .getByRole("radio", { name: /drain/i })
      .querySelector('[data-slot="radio-card-icon-well"]');
    expect(well?.className).toContain("text-muted");
    expect(well?.className).toContain("shadow-hairline-inset");
  });
});
