// Suite: stacked palette focus restore
// Invariant: popping a stacked view can return keyboard focus to the visible frame input.
// Boundary IN: focusActivePaletteFrame.
// Boundary OUT: CommandDialog focus trap and view-stack pop.
import { describe, expect, it } from "vitest";

import { focusActivePaletteFrame } from "../os-command-palette-focus";

describe("stacked palette focus restore", () => {
  it("Should focus the visible frame command input [RD0054]", () => {
    const frame = document.createElement("div");
    const input = document.createElement("input");
    input.setAttribute("data-slot", "command-input");
    frame.append(input);
    document.body.append(frame);

    focusActivePaletteFrame(frame);
    expect(document.activeElement).toBe(input);

    frame.remove();
  });

  it("Should do nothing when the frame has no command input", () => {
    const frame = document.createElement("div");
    document.body.append(frame);
    const previous = document.activeElement;

    focusActivePaletteFrame(frame);
    expect(document.activeElement).toBe(previous);

    frame.remove();
  });
});
