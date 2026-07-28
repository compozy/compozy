import { describe, expect, it } from "vitest";

import {
  DIALOG_TOUCH_TARGET_CLASS,
  DIALOG_TOUCH_TARGET_SEGMENTS_CLASS,
  DIALOG_TOUCH_TARGET_SQUARE_CLASS,
  dialogShellClass,
  type DialogShellSize,
} from "../dialog-shell";

const SIZES: DialogShellSize[] = ["sm", "md", "lg", "xl"];

describe("dialogShellClass", () => {
  it("Should emit the modal width token for every size", () => {
    for (const size of SIZES) {
      expect(dialogShellClass(size)).toContain(`w-(--width-modal-${size})`);
    }
  });

  it("Should never emit a raw Tailwind max-width scale value", () => {
    for (const size of SIZES) {
      // `max-w-2xl` / `max-w-xl` / `max-w-3xl` / `max-w-120` are the ad-hoc widths
      // this helper replaces. `max-w-full` is a keyword, not a scale step, and is
      // how the bottom surface goes full bleed.
      const scaleWidths = dialogShellClass(size)
        .split(" ")
        .filter(token => /(^|:)max-w-/.test(token))
        .filter(token => !/max-w-(\[|\(|full$|none$)/.test(token));
      expect(scaleWidths).toEqual([]);
    }
  });

  it("Should cap height against the viewport for every size", () => {
    for (const size of SIZES) {
      expect(dialogShellClass(size)).toContain("calc(100vh-2rem)");
    }
  });

  it("Should map each size to its documented height token", () => {
    expect(dialogShellClass("sm")).toContain("var(--height-modal-tall)");
    expect(dialogShellClass("md")).toContain("var(--height-modal-md)");
    expect(dialogShellClass("lg")).toContain("var(--height-modal-tall)");
    expect(dialogShellClass("xl")).toContain("var(--height-modal-xl)");
  });

  it("Should pin the host height only when fill is requested", () => {
    expect(dialogShellClass("md")).not.toContain("h-(--height-modal-md)");
    expect(dialogShellClass("md", { fill: true })).toContain("h-(--height-modal-md)");
    expect(dialogShellClass("xl", { fill: true })).toContain("h-(--height-modal-xl)");
  });

  it("Should let shell rows shrink below their content width", () => {
    expect(dialogShellClass("lg")).toContain("grid-cols-[minmax(0,1fr)]");
  });

  it("Should become a full-width bottom surface at 760px and below", () => {
    for (const size of SIZES) {
      const shell = dialogShellClass(size, { fill: true });
      // Bottom-anchored, centering transform dropped.
      expect(shell).toContain("max-[760px]:bottom-0");
      expect(shell).toContain("max-[760px]:top-auto");
      expect(shell).toContain("max-[760px]:translate-x-0");
      expect(shell).toContain("max-[760px]:translate-y-0");
      // Full-bleed width is what keeps the page free of horizontal scroll.
      expect(shell).toContain("max-[760px]:w-full");
      expect(shell).toContain("max-[760px]:max-w-full");
      // A pinned height must not survive into the bottom surface.
      expect(shell).toContain("max-[760px]:h-auto");
      expect(shell).toContain("max-[760px]:rounded-b-none");
    }
  });

  it("Should express every touch target from the 44px button token", () => {
    for (const target of [
      DIALOG_TOUCH_TARGET_CLASS,
      DIALOG_TOUCH_TARGET_SQUARE_CLASS,
      DIALOG_TOUCH_TARGET_SEGMENTS_CLASS,
    ]) {
      expect(target).toContain("max-[760px]:");
      expect(target).toContain("(--height-button-cta-lg)");
    }
    // Segment targets must be a single literal so Tailwind's source scan sees them.
    expect(DIALOG_TOUCH_TARGET_SEGMENTS_CLASS).toContain("[&>[data-slot=pill-group-item]]:");
  });
});
