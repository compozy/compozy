import type { LayoutNode, PixelSize, WindowId, WindowMinimums } from "./window-manager-types";

const EMPTY_MINIMUM: PixelSize = { width: 0, height: 0 };

function nonNegativeInteger(value: number): number {
  return Number.isFinite(value) ? Math.max(0, Math.round(value)) : 0;
}

export function minimumForWindow(windowId: WindowId, minimums: WindowMinimums): PixelSize {
  const minimum = minimums[windowId];
  if (minimum === undefined) return EMPTY_MINIMUM;
  return {
    width: nonNegativeInteger(minimum.width),
    height: nonNegativeInteger(minimum.height),
  };
}

export function minimumForNode(
  node: LayoutNode,
  innerGap: number,
  minimums: WindowMinimums
): PixelSize {
  if (node.kind === "leaf") return minimumForWindow(node.windowId, minimums);
  if (node.kind === "stack") {
    return node.windowIds.reduce<PixelSize>((maximum, windowId) => {
      const minimum = minimumForWindow(windowId, minimums);
      return {
        width: Math.max(maximum.width, minimum.width),
        height: Math.max(maximum.height, minimum.height),
      };
    }, EMPTY_MINIMUM);
  }
  const children = node.children.map(child => minimumForNode(child, innerGap, minimums));
  const gapTotal = innerGap * Math.max(0, children.length - 1);
  if (node.axis === "horizontal") {
    return {
      width: children.reduce((sum, child) => sum + child.width, gapTotal),
      height: children.reduce((maximum, child) => Math.max(maximum, child.height), 0),
    };
  }
  return {
    width: children.reduce((maximum, child) => Math.max(maximum, child.width), 0),
    height: children.reduce((sum, child) => sum + child.height, gapTotal),
  };
}
