import type { PixelRect, WindowManagerGapsConfig } from "./window-manager-types";

type OuterGaps = Omit<WindowManagerGapsConfig, "inner">;

function nonNegativeInteger(value: number): number {
  return Number.isFinite(value) ? Math.max(0, Math.round(value)) : 0;
}

function pixelRect(rect: PixelRect): PixelRect {
  return {
    x: Number.isFinite(rect.x) ? Math.round(rect.x) : 0,
    y: Number.isFinite(rect.y) ? Math.round(rect.y) : 0,
    w: nonNegativeInteger(rect.w),
    h: nonNegativeInteger(rect.h),
  };
}

/** The one pixel coordinate box used for tiled preview, normalization, and projection. */
export function windowManagerLayoutArea(area: PixelRect, gaps: OuterGaps): PixelRect {
  const rect = pixelRect(area);
  const left = Math.min(nonNegativeInteger(gaps.left), rect.w);
  const right = Math.min(nonNegativeInteger(gaps.right), rect.w - left);
  const top = Math.min(nonNegativeInteger(gaps.top), rect.h);
  const bottom = Math.min(nonNegativeInteger(gaps.bottom), rect.h - top);
  return {
    x: rect.x + left,
    y: rect.y + top,
    w: rect.w - left - right,
    h: rect.h - top - bottom,
  };
}
