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

/**
 * Splits the inner gap across every interior edge of a tile that owns a whole
 * area zone. Sibling splits already reserve a full gap between children, but two
 * independent group frames only meet on a shared fraction — each side gives back
 * half so the boundary reads as exactly one gap, while edges flush with the
 * layout area stay flush against the outer gaps.
 */
export function windowManagerTileInset(
  rect: PixelRect,
  area: PixelRect,
  innerGap: number
): PixelRect {
  const gap = nonNegativeInteger(innerGap);
  if (gap === 0) return rect;
  const leading = Math.ceil(gap / 2);
  const trailing = gap - leading;
  const left = rect.x > area.x ? leading : 0;
  const top = rect.y > area.y ? leading : 0;
  const right = rect.x + rect.w < area.x + area.w ? trailing : 0;
  const bottom = rect.y + rect.h < area.y + area.h ? trailing : 0;
  return {
    x: rect.x + left,
    y: rect.y + top,
    w: Math.max(0, rect.w - left - right),
    h: Math.max(0, rect.h - top - bottom),
  };
}
