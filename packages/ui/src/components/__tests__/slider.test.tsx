import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Slider } from "../slider";

describe("Slider", () => {
  it("Should label the control that carries the value, not the group around it", () => {
    render(<Slider aria-label="History steps" defaultValue={50} max={500} min={1} />);

    const control = screen.getByLabelText("History steps");
    expect(control).toHaveAttribute("type", "range");
    expect(control).toHaveAttribute("min", "1");
    expect(control).toHaveAttribute("max", "500");
    expect(control).toHaveValue("50");
  });

  it("Should render one thumb for a single value and one per entry for a range", () => {
    const { container, rerender } = render(<Slider aria-label="History steps" defaultValue={50} />);
    expect(container.querySelectorAll("[data-slot=slider-thumb]")).toHaveLength(1);

    rerender(<Slider aria-label="Bounds" defaultValue={[20, 80]} />);
    expect(container.querySelectorAll("[data-slot=slider-thumb]")).toHaveLength(2);
  });

  it("Should give every range thumb a distinct accessible name", () => {
    render(<Slider aria-label="Bounds" defaultValue={[20, 80]} />);

    expect(screen.getByLabelText("Bounds: Minimum")).toBeInTheDocument();
    expect(screen.getByLabelText("Bounds: Maximum")).toBeInTheDocument();
  });

  it("Should step with the arrow keys and jump with Home and End", async () => {
    const onValueChange = vi.fn();
    const user = userEvent.setup();
    render(
      <Slider
        aria-label="History steps"
        defaultValue={50}
        max={500}
        min={1}
        onValueChange={onValueChange}
      />
    );

    screen.getByLabelText("History steps").focus();

    await user.keyboard("{ArrowRight}");
    expect(onValueChange).toHaveBeenLastCalledWith(51, expect.anything());

    await user.keyboard("{ArrowLeft}");
    expect(onValueChange).toHaveBeenLastCalledWith(50, expect.anything());

    await user.keyboard("{Home}");
    expect(onValueChange).toHaveBeenLastCalledWith(1, expect.anything());

    await user.keyboard("{End}");
    expect(onValueChange).toHaveBeenLastCalledWith(500, expect.anything());
  });

  it("Should not move while disabled", async () => {
    const onValueChange = vi.fn();
    const user = userEvent.setup();
    render(
      <Slider
        aria-label="History steps"
        defaultValue={50}
        disabled
        max={500}
        min={1}
        onValueChange={onValueChange}
      />
    );

    await user.keyboard("{Tab}{ArrowRight}");

    expect(onValueChange).not.toHaveBeenCalled();
  });

  it("Should carry the data-slot contract for every part", () => {
    const { container } = render(<Slider aria-label="History steps" defaultValue={50} />);

    expect(container.querySelector("[data-slot=slider]")).not.toBeNull();
    expect(container.querySelector("[data-slot=slider-track]")).not.toBeNull();
    expect(container.querySelector("[data-slot=slider-range]")).not.toBeNull();
    expect(container.querySelector("[data-slot=slider-thumb]")).not.toBeNull();
  });
});
