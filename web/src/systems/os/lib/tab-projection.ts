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

/**
 * Height of the member switcher a collapsed split shows above its member: the member's rect
 * starts below it, so drop zones and focus geometry match what is on screen.
 */
export const MEMBER_SWITCHER_PX = 36;

function belowSwitcher(rect: PixelRect): PixelRect {
  const offset = Math.min(MEMBER_SWITCHER_PX, rect.h);
  return { x: rect.x, y: rect.y + offset, w: rect.w, h: rect.h - offset };
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
  const memberRect = belowSwitcher(rect);
  for (const entry of entries) {
    const minimum = minimumForWindow(entry.windowId, context.minimums);
    if (!sizeFits(memberRect, minimum)) {
      context.diagnostics.push({
        code: "minimum-unmet",
        nodeId: entry.nodeId,
        windowId: entry.windowId,
        required: minimum,
        available: { width: memberRect.w, height: memberRect.h },
      });
    }
    context.windows.push({
      windowId: entry.windowId,
      nodeId: entry.nodeId,
      rect: memberRect,
      shown: visible && entry.windowId === shownWindowId,
      parentAxis: entry.parentAxis,
    });
  }
}

/**
 * Whether the split still lays out side by side once `shift` pixels move across one boundary,
 * weighted and allocated exactly as a seam preview or `layout.resize` of that shift would be:
 * the same weight arithmetic as `applySeamPreviewToTab`, the same largest-remainder allocation
 * and the same fit test that decides between side by side and one member at a time.
 */
function fitsShifted(
  node: LayoutSplitNode,
  rect: PixelRect,
  boundaryIndex: number,
  shift: number,
  axisSpan: number,
  context: ProjectionContext
): boolean {
  const delta = shift / axisSpan;
  const weights = [...node.weights];
  weights[boundaryIndex] = (weights[boundaryIndex] ?? 0) + delta;
  weights[boundaryIndex + 1] = (weights[boundaryIndex + 1] ?? 0) - delta;
  const shifted: LayoutSplitNode = { ...node, weights };
  return childrenFit(shifted, splitRects(shifted, rect, context.innerGap), context);
}

/**
 * The farthest whole-pixel shift toward `wanted` the split admits. `wanted` brings one side to its
 * least extent, but a share that lands on a half pixel can be allocated to the other side, so the
 * bound backs off pixel by pixel until the split still lays out side by side. A shift of 0 is the
 * current layout, which fits.
 */
function admittedShift(
  node: LayoutSplitNode,
  rect: PixelRect,
  boundaryIndex: number,
  wanted: number,
  axisSpan: number,
  context: ProjectionContext
): number {
  // A split with no span cannot be dragged: `seamWeightDelta` moves nothing either.
  if (axisSpan <= 0) return 0;
  let shift = wanted;
  while (shift !== 0 && !fitsShifted(node, rect, boundaryIndex, shift, axisSpan, context)) {
    shift -= Math.sign(shift);
  }
  return shift;
}

function seamForBoundary(
  node: LayoutSplitNode,
  rect: PixelRect,
  childRects: readonly PixelRect[],
  boundaryIndex: number,
  context: ProjectionContext
): ProjectedSeam | null {
  const leadingNode = node.children[boundaryIndex];
  const trailingNode = node.children[boundaryIndex + 1];
  const leading = childRects[boundaryIndex];
  const trailing = childRects[boundaryIndex + 1];
  if (!leadingNode || !trailingNode || !leading || !trailing) return null;
  // A branch may shrink until it would no longer fit, collapsed or not: a collapsed branch keeps
  // room for its member switcher above its member.
  const leadingMinimum = minimumExtent(leadingNode, leading, node.axis, context);
  const trailingMinimum = minimumExtent(trailingNode, trailing, node.axis, context);
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
  // The bounds are the shifts the split still lays out side by side at, by the same rule the
  // projection applies to the previewed and the committed weights, so a drag stopped at either
  // bound never shows the split one member at a time.
  const extent = (childRect: PixelRect) => (horizontal ? childRect.w : childRect.h);
  const bound = (wanted: number) =>
    admittedShift(node, rect, boundaryIndex, wanted, axisSpan, context);
  const leadingShift = bound(leadingMinimum - extent(leading));
  const trailingShift = bound(extent(trailing) - trailingMinimum);
  if (horizontal) {
    const start = leading.x + leading.w;
    const width = Math.max(0, trailing.x - start);
    const value = start + width / 2;
    return {
      ...shared,
      orientation: "vertical",
      rect: { x: start, y: leading.y, w: width, h: leading.h },
      value,
      minValue: value + leadingShift,
      maxValue: value + trailingShift,
    };
  }
  const start = leading.y + leading.h;
  const height = Math.max(0, trailing.y - start);
  const value = start + height / 2;
  return {
    ...shared,
    orientation: "horizontal",
    rect: { x: leading.x, y: start, w: leading.w, h: height },
    value,
    minValue: value + leadingShift,
    maxValue: value + trailingShift,
  };
}

function sizeFits(rect: PixelRect, minimum: PixelSize): boolean {
  return rect.w >= minimum.width && rect.h >= minimum.height;
}

/** Whether a split shown one member at a time fits: every member, below the switcher. */
function collapsedFits(node: LayoutNode, rect: PixelRect, minimums: WindowMinimums): boolean {
  const member = belowSwitcher(rect);
  return descendantWindowIds(node).every(windowId =>
    sizeFits(member, minimumForWindow(windowId, minimums))
  );
}

/**
 * Whether a node fits a rect as the projection would lay it out: a window needs its own minimum;
 * a split lays out by its weights when every child fits, and otherwise shows one member at a time,
 * which needs every member's minimum below the member switcher. Using this rather than the full
 * laid-out minimum lets only the split that cannot fit collapse instead of every ancestor.
 */
function fitsIn(node: LayoutNode, rect: PixelRect, context: ProjectionContext): boolean {
  if (node.kind === "leaf")
    return sizeFits(rect, minimumForWindow(node.windowId, context.minimums));
  if (!context.compact && childrenFit(node, splitRects(node, rect, context.innerGap), context)) {
    return true;
  }
  return collapsedFits(node, rect, context.minimums);
}

function childrenFit(
  node: LayoutSplitNode,
  childRects: readonly PixelRect[],
  context: ProjectionContext
): boolean {
  return node.children.every((child, index) => {
    const childRect = childRects[index];
    return childRect !== undefined && fitsIn(child, childRect, context);
  });
}

/**
 * The least extent along `axis` a node can take within `rect`'s cross size: a window's minimum,
 * or the least extent at which a split still fits, laid out or one member at a time.
 */
function minimumExtent(
  node: LayoutNode,
  rect: PixelRect,
  axis: LayoutAxis,
  context: ProjectionContext
): number {
  const horizontal = axis === "horizontal";
  if (node.kind === "leaf") {
    const minimum = minimumForWindow(node.windowId, context.minimums);
    return horizontal ? minimum.width : minimum.height;
  }
  const members = descendantWindowIds(node).map(windowId =>
    minimumForWindow(windowId, context.minimums)
  );
  // Shown one member at a time, the split needs its widest member and, above the tallest, the
  // switcher: the upper bound of the search, and the answer when the cross size never fits.
  const collapsed = horizontal
    ? Math.max(0, ...members.map(minimum => minimum.width))
    : Math.max(0, ...members.map(minimum => minimum.height)) + MEMBER_SWITCHER_PX;
  const sized = (extent: number): PixelRect =>
    horizontal ? { ...rect, w: extent } : { ...rect, h: extent };
  if (!fitsIn(node, sized(collapsed), context)) return collapsed;
  let low = 0;
  let high = collapsed;
  while (low < high) {
    const middle = Math.floor((low + high) / 2);
    if (fitsIn(node, sized(middle), context)) high = middle;
    else low = middle + 1;
  }
  return high;
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
    const seam = seamForBoundary(node, rect, childRects, index, context);
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
