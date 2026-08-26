// Suite: color picker primitive
// Invariant: the picker exposes accessible slider controls bound to the given
// hex value and reports changes as hex through onChange.
// Boundary IN: the ColorPicker wrapper contract over react-colorful.
// Boundary OUT: hex validation and swatches (SymbolPicker color section suite).

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ColorPicker } from "../color-picker";

describe("ColorPicker", () => {
  it("Should expose saturation and hue as accessible sliders", () => {
    render(<ColorPicker value="#4ea7fc" onChange={() => {}} />);
    expect(screen.getByRole("slider", { name: "Color" })).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: "Hue" })).toBeInTheDocument();
  });

  it("Should emit hex values when the hue slider moves by keyboard", () => {
    const onChange = vi.fn();
    render(<ColorPicker value="#4ea7fc" onChange={onChange} />);
    // react-colorful reads the legacy keyCode field, which real key events carry.
    fireEvent.keyDown(screen.getByRole("slider", { name: "Hue" }), {
      key: "ArrowRight",
      keyCode: 39,
    });
    expect(onChange).toHaveBeenCalledWith(expect.stringMatching(/^#[0-9a-f]{6}$/));
  });
});
