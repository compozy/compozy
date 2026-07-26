import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useReducedMotion } from "../use-reduced-motion";

function ReducedMotionProbe() {
  return <output>{useReducedMotion() ? "reduced" : "default"}</output>;
}

describe("useReducedMotion", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("reads the current reduced-motion preference with modern listeners", () => {
    const addEventListener = vi.fn();
    const removeEventListener = vi.fn();
    vi.stubGlobal(
      "matchMedia",
      vi.fn().mockReturnValue({
        matches: true,
        addEventListener,
        removeEventListener,
      })
    );

    render(<ReducedMotionProbe />);

    expect(screen.getByText("reduced")).toBeDefined();
    expect(addEventListener).toHaveBeenCalledWith("change", expect.any(Function));
  });

  it("falls back to legacy matchMedia listeners", () => {
    const addListener = vi.fn();
    const removeListener = vi.fn();
    vi.stubGlobal(
      "matchMedia",
      vi.fn().mockReturnValue({
        matches: true,
        addListener,
        removeListener,
      })
    );

    const { unmount } = render(<ReducedMotionProbe />);

    expect(screen.getByText("reduced")).toBeDefined();
    expect(addListener).toHaveBeenCalledWith(expect.any(Function));

    unmount();

    expect(removeListener).toHaveBeenCalledWith(expect.any(Function));
  });
});
