import { buildWindowManagerMinimums, projectLayout, type LayoutProjection } from "@/systems/os";
import type { PixelRect, WindowManagerConfig } from "@/systems/os";

import { LAYOUT_CANVAS_REFERENCE } from "./window-manager-layout-reference";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerNormalizedRect,
} from "./window-manager-layout-types";

export function layoutCanvasProjection(
  document: WindowManagerLayoutDocument,
  desktop: WindowManagerLayoutDesktop,
  config: WindowManagerConfig,
  revision: number
): LayoutProjection {
  return projectLayout({
    revision,
    desktop,
    workArea: LAYOUT_CANVAS_REFERENCE,
    gaps: config.gaps,
    minimums: buildWindowManagerMinimums(document),
    focusedWindowId: null,
  });
}

/** Reference pixels → CSS percentages, so the canvas scales with its container. */
export function canvasPercentStyle(rect: PixelRect) {
  return {
    left: `${(rect.x / LAYOUT_CANVAS_REFERENCE.w) * 100}%`,
    top: `${(rect.y / LAYOUT_CANVAS_REFERENCE.h) * 100}%`,
    width: `${(rect.w / LAYOUT_CANVAS_REFERENCE.w) * 100}%`,
    height: `${(rect.h / LAYOUT_CANVAS_REFERENCE.h) * 100}%`,
  };
}

export function normalizedPercentStyle(rect: WindowManagerNormalizedRect) {
  return {
    left: `${rect.x * 100}%`,
    top: `${rect.y * 100}%`,
    width: `${rect.w * 100}%`,
    height: `${rect.h * 100}%`,
  };
}

export function rectsOverlap(
  a: WindowManagerNormalizedRect,
  b: WindowManagerNormalizedRect
): boolean {
  const epsilon = 1e-6;
  return (
    a.x < b.x + b.w - epsilon &&
    b.x < a.x + a.w - epsilon &&
    a.y < b.y + b.h - epsilon &&
    b.y < a.y + a.h - epsilon
  );
}

/** Group ids the daemon would reject with `topology.group_overlap`. */
export function overlappingGroupIds(desktop: WindowManagerLayoutDesktop): Set<string> {
  const overlapping = new Set<string>();
  const groups = desktop.groups;
  for (let index = 0; index < groups.length; index += 1) {
    for (let peer = index + 1; peer < groups.length; peer += 1) {
      const left = groups[index];
      const right = groups[peer];
      if (!left || !right || !rectsOverlap(left.frame, right.frame)) continue;
      overlapping.add(left.id);
      overlapping.add(right.id);
    }
  }
  return overlapping;
}
