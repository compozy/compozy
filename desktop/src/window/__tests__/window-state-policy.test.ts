import { describe, expect, it } from "vitest";

import { parseWindowState, restoreWindowBounds } from "../window-state-policy";

const display = { x: 0, y: 25, width: 1920, height: 1055 };
const centeredDefault = { x: 320, y: 152, width: 1280, height: 800 };

// Invariant: restored geometry is usable on a connected display and never strands the window off-screen.
describe("window-state policy", () => {
  it("Should preserve usable geometry with a visible edge", () => {
    const state = { x: 100, y: 100, width: 1280, height: 800, maximized: false };
    expect(restoreWindowBounds(state, [display])).toEqual(state);
  });

  it.each([
    { x: 3000, y: 0, width: 1280, height: 800, maximized: false },
    { x: 100, y: 100, width: 899, height: 800, maximized: false },
    { x: 100, y: 100, width: 1280, height: 599, maximized: false },
  ])("Should recover unusable geometry %#", state => {
    expect(restoreWindowBounds(state, [display])).toEqual(centeredDefault);
  });

  it("Should reject corrupt persisted shapes", () => {
    expect(
      parseWindowState({ x: "100", y: 0, width: 1280, height: 800, maximized: false })
    ).toBeNull();
  });
});
