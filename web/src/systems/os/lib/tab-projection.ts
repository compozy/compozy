import { descendantWindowIds, minimumForNode, minimumForWindow } from "./layout-minimums";
import type {
  CollapsedSplit,
  ProjectedSeam,
  ProjectedWindow,
  TabProjection,
  TabProjectionDiagnostic,
  TabProjectionInput,
  WindowMinimums,
} from "./tab-projection-types";
import { windowManagerLayoutArea } from "./window-manager-layout-area";
import type {
  LayoutAxis,
  LayoutNode,
  LayoutSplitNode,
  PixelRect,
  PixelSize,
  WindowId,
} from "./window-manager-types";

interface ProjectionContext {
  innerGap: number;
  minimums: WindowMinimums;
  focusOrder: readonly WindowId[];
  compact: boolean;
  windows: ProjectedWindow[];
  seams: ProjectedSeam[];
  collapsed: CollapsedSplit[];
  diagnostics: TabProjectionDiagnostic[];
}

function finite(value: number, fallback = 0): number {
  return Number.isFinite(value) ? value : fallback;
}

function nonNegativeInteger(value: number): number {
  return Math.max(0, Math.round(finite(value)));
}

function pixelRect(rect: PixelRect): PixelRect {
  return {
    x: Math.round(finite(rect.x)),
    y: Math.round(finite(rect.y)),
    w: nonNegativeInteger(rect.w),
    h: nonNegativeInteger(rect.h),
  };
}

/** Weights as shares of one; malformed weights fall back to equal shares. */
export function normalizedWeights(node: LayoutSplitNode): number[] {
  const valid =
    node.weights.length === node.children.length &&
    node.weights.every(weight => weight > 0 && Number.isFinite(weight));
  if (!valid) return node.children.map(() => 1 / Math.max(1, node.children.length));
  const total = node.weights.reduce((sum, weight) => sum + weight, 0);
  return node.weights.map(weight => weight / total);
}

/** Largest-remainder allocation keeps every split integral and exactly fills its axis. */
function allocatePixels(total: number, weights: readonly number[]): number[] {
  if (weights.length === 0) return [];
  const available = nonNegativeInteger(total);
  const raw = weights.map(weight => available * weight);
  const allocation = raw.map(value => Math.floor(value));
  let remainder = available - allocation.reduce((sum, value) => sum + value, 0);
  const order = raw
    .map((value, index) => ({ index, fraction: value - Math.floor(value) }))
    .sort((a, b) => b.fraction - a.fraction || a.index - b.index);
  for (const entry of order) {
    if (remainder <= 0) break;
    allocation[entry.index] = (allocation[entry.index] ?? 0) + 1;
    remainder -= 1;
  }
  return allocation;
}

function splitRects(node: LayoutSplitNode, rect: PixelRect, gap: number): PixelRect[] {
  const gapTotal = gap * Math.max(0, node.children.length - 1);
  const axisLength = node.axis === "horizontal" ? rect.w : rect.h;
  const sizes = allocatePixels(Math.max(0, axisLength - gapTotal), normalizedWeights(node));
  let cursor = node.axis === "horizontal" ? rect.x : rect.y;
  return sizes.map(size => {
    const childRect =
      node.axis === "horizontal"
        ? { x: cursor, y: rect.y, w: size, h: rect.h }
        : { x: rect.x, y: cursor, w: rect.w, h: size };
    cursor += size + gap;
    return childRect;
  });
}

/** The member a collapsed split shows: the one this client focused last, else its first window. */
function shownMember(windowIds: readonly WindowId[], focusOrder: readonly WindowId[]): WindowId {
  const members = new Set(windowIds);
  return focusOrder.find(windowId => members.has(windowId)) ?? windowIds[0] ?? "";
}

function leafEntries(
  node: LayoutNode,
  parentAxis: LayoutAxis | null
): Array<{ windowId: WindowId; nodeId: string; parentAxis: LayoutAxis | null }> {
  if (node.kind === "leaf") return [{ windowId: node.windowId, nodeId: node.id, parentAxis }];
  return node.children.flatMap(child => leafEntries(child, node.axis));
}

function projectCollapsed(
  node: LayoutSplitNode,
  rect: PixelRect,
  context: ProjectionContext,
  parentAxis: LayoutAxis | null,
  visible: boolean
): void {
  const entries = leafEntries(node, parentAxis);
  const windowIds = entries.map(entry => entry.windowId);
  const shownWindowId = shownMember(windowIds, context.focusOrder);
  context.collapsed.push({ splitId: node.id, windowIds, shownWindowId, rect });
  for (const entry of entries) {
    context.windows.push({
      windowId: entry.windowId,
      nodeId: entry.nodeId,
      rect,
      shown: visible && entry.windowId === shownWindowId,
      parentAxis: entry.parentAxis,
    });
  }
}

function seamForBoundary(
  node: LayoutSplitNode,
  childRects: readonly PixelRect[],
  boundaryIndex: number,
  context: ProjectionContext
): ProjectedSeam | null {
  const leadingNode = node.children[boundaryIndex];
  const trailingNode = node.children[boundaryIndex + 1];
  const leading = childRects[boundaryIndex];
  const trailing = childRects[boundaryIndex + 1];
  if (!leadingNode || !trailingNode || !leading || !trailing) return null;
  // A branch may shrink until it would collapse, not only until its split stops fitting.
  const leadingMinimum = collapsedFloor(leadingNode, context.minimums);
  const trailingMinimum = collapsedFloor(trailingNode, context.minimums);
  const weights = normalizedWeights(node);
  const horizontal = node.axis === "horizontal";
  const axisSpan = childRects.reduce(
    (sum, childRect) => sum + (horizontal ? childRect.w : childRect.h),
    0
  );
  const shared = {
    id: `${node.id}:${boundaryIndex}`,
    splitId: node.id,
    boundaryIndex,
    axisSpan,
    leadingWeight: weights[boundaryIndex] ?? 0,
    trailingWeight: weights[boundaryIndex + 1] ?? 0,
    leadingWindowIds: descendantWindowIds(leadingNode),
    trailingWindowIds: descendantWindowIds(trailingNode),
  };
  if (horizontal) {
    const start = leading.x + leading.w;
    const width = Math.max(0, trailing.x - start);
    return {
      ...shared,
      orientation: "vertical",
      rect: { x: start, y: leading.y, w: width, h: leading.h },
      value: start + width / 2,
      minValue: leading.x + leadingMinimum.width + width / 2,
      maxValue: trailing.x + trailing.w - trailingMinimum.width - width / 2,
    };
  }
  const start = leading.y + leading.h;
  const height = Math.max(0, trailing.y - start);
  return {
    ...shared,
    orientation: "horizontal",
    rect: { x: leading.x, y: start, w: leading.w, h: height },
    value: start + height / 2,
    minValue: leading.y + leadingMinimum.height + height / 2,
    maxValue: trailing.y + trailing.h - trailingMinimum.height - height / 2,
  };
}

/**
 * The least space a node can occupy: a window needs its own minimum, while a split can always
 * fall back to showing one member at a time, so it needs only its largest member's minimum.
 * Using this floor lets only the split that cannot fit collapse instead of every ancestor.
 */
function collapsedFloor(node: LayoutNode, minimums: WindowMinimums): PixelSize {
  if (node.kind === "leaf") return minimumForWindow(node.windowId, minimums);
  return descendantWindowIds(node).reduce<PixelSize>(
    (floor, windowId) => {
      const minimum = minimumForWindow(windowId, minimums);
      return {
        width: Math.max(floor.width, minimum.width),
        height: Math.max(floor.height, minimum.height),
      };
    },
    { width: 0, height: 0 }
  );
}

function childrenFit(
  node: LayoutSplitNode,
  childRects: readonly PixelRect[],
  context: ProjectionContext
): boolean {
  return node.children.every((child, index) => {
    const childRect = childRects[index];
    if (childRect === undefined) return false;
    const floor = collapsedFloor(child, context.minimums);
    return childRect.w >= floor.width && childRect.h >= floor.height;
  });
}

function projectNode(
  node: LayoutNode,
  rect: PixelRect,
  context: ProjectionContext,
  parentAxis: LayoutAxis | null,
  visible: boolean
): void {
  if (node.kind === "leaf") {
    const minimum = minimumForWindow(node.windowId, context.minimums);
    if (rect.w < minimum.width || rect.h < minimum.height) {
      context.diagnostics.push({
        code: "minimum-unmet",
        nodeId: node.id,
        windowId: node.windowId,
        required: minimum,
        available: { width: rect.w, height: rect.h },
      });
    }
    context.windows.push({
      windowId: node.windowId,
      nodeId: node.id,
      rect,
      shown: visible,
      parentAxis,
    });
    return;
  }
  const childRects = splitRects(node, rect, context.innerGap);
  if (context.compact || !childrenFit(node, childRects, context)) {
    context.diagnostics.push({
      code: "adaptive-stack",
      nodeId: node.id,
      required: minimumForNode(node, context.innerGap, context.minimums),
      available: { width: rect.w, height: rect.h },
    });
    projectCollapsed(node, rect, context, parentAxis, visible);
    return;
  }
  node.children.forEach((child, index) => {
    const childRect = childRects[index];
    if (childRect !== undefined) projectNode(child, childRect, context, node.axis, visible);
  });
  for (let index = 0; index < node.children.length - 1; index += 1) {
    const seam = seamForBoundary(node, childRects, index, context);
    if (seam !== null) context.seams.push(seam);
  }
}

/**
 * Projects one tab's split tree into pixel rects without changing it: windows laid out from the
 * weights, the seams between them, and the splits that show one member at a time because their
 * members cannot fit (or because the viewport is compact). Weights are never rewritten.
 */
export function projectTab(input: TabProjectionInput): TabProjection {
  const workArea = pixelRect(input.workArea);
  const area = windowManagerLayoutArea(workArea, input.gaps);
  const context: ProjectionContext = {
    innerGap: nonNegativeInteger(input.gaps.inner),
    minimums: input.minimums ?? {},
    focusOrder: input.focusOrder ?? [],
    compact: input.compact ?? false,
    windows: [],
    seams: [],
    collapsed: [],
    diagnostics: [],
  };
  projectNode(input.tab.root, area, context, null, true);
  return {
    tabId: input.tab.id,
    workArea,
    windows: context.windows,
    seams: context.seams,
    collapsed: context.collapsed,
    diagnostics: context.diagnostics,
  };
}
