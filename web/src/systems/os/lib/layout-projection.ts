import type {
  LayoutAxis,
  LayoutGroup,
  LayoutNode,
  LayoutProjection,
  LayoutProjectionDiagnostic,
  LayoutProjectionInput,
  LayoutSplitNode,
  PixelPoint,
  PixelRect,
  PixelSize,
  ProjectedSeam,
  ProjectedStack,
  ProjectedWindow,
  WindowId,
  WindowMinimums,
} from "./window-manager-types";
import { windowManagerLayoutArea } from "./window-manager-layout-area";

const EMPTY_MINIMUM: PixelSize = { width: 0, height: 0 };

interface ProjectionCollector {
  windows: ProjectedWindow[];
  stacks: ProjectedStack[];
  seams: ProjectedSeam[];
  diagnostics: LayoutProjectionDiagnostic[];
}

interface NodeProjectionContext {
  groupId: string;
  innerGap: number;
  minimums: WindowMinimums;
  focusedWindowId: WindowId | null;
  collector: ProjectionCollector;
}

export interface FloatingRectCommit {
  proposedRect: PixelRect;
  workArea: PixelRect;
  pointer?: PixelPoint;
  grabOffset?: PixelPoint;
  titleBarHeight?: number;
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

function clampUnit(value: number): number {
  return Math.min(1, Math.max(0, finite(value)));
}

function isValidNormalizedRect(frame: LayoutGroup["frame"]): boolean {
  return (
    Number.isFinite(frame.x) &&
    Number.isFinite(frame.y) &&
    Number.isFinite(frame.w) &&
    Number.isFinite(frame.h) &&
    frame.x >= 0 &&
    frame.y >= 0 &&
    frame.w > 0 &&
    frame.h > 0 &&
    frame.x + frame.w <= 1 &&
    frame.y + frame.h <= 1
  );
}

function projectGroupFrame(frame: LayoutGroup["frame"], area: PixelRect): PixelRect {
  const leftFraction = clampUnit(frame.x);
  const topFraction = clampUnit(frame.y);
  const rightFraction = clampUnit(frame.x + Math.max(0, finite(frame.w)));
  const bottomFraction = clampUnit(frame.y + Math.max(0, finite(frame.h)));
  const left = area.x + Math.round(area.w * leftFraction);
  const top = area.y + Math.round(area.h * topFraction);
  const right = area.x + Math.round(area.w * Math.max(leftFraction, rightFraction));
  const bottom = area.y + Math.round(area.h * Math.max(topFraction, bottomFraction));
  return { x: left, y: top, w: right - left, h: bottom - top };
}

function minimumForWindow(windowId: WindowId, minimums: WindowMinimums): PixelSize {
  const minimum = minimums[windowId];
  if (minimum === undefined) return EMPTY_MINIMUM;
  return {
    width: nonNegativeInteger(minimum.width),
    height: nonNegativeInteger(minimum.height),
  };
}

function minimumForNode(node: LayoutNode, innerGap: number, minimums: WindowMinimums): PixelSize {
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

function descendantEntries(
  node: LayoutNode,
  parentAxis: LayoutAxis | null = null
): Array<{ windowId: WindowId; nodeId: string; parentAxis: LayoutAxis | null }> {
  if (node.kind === "leaf") return [{ windowId: node.windowId, nodeId: node.id, parentAxis }];
  if (node.kind === "stack") {
    return node.windowIds.map(windowId => ({ windowId, nodeId: node.id, parentAxis }));
  }
  return node.children.flatMap(child => descendantEntries(child, node.axis));
}

function descendantWindowIds(node: LayoutNode): WindowId[] {
  return descendantEntries(node).map(entry => entry.windowId);
}

function explicitActiveWindow(node: LayoutNode): WindowId | null {
  if (node.kind === "leaf") return null;
  if (node.kind === "stack") return node.windowIds.includes(node.activeId) ? node.activeId : null;
  for (const child of node.children) {
    const active = explicitActiveWindow(child);
    if (active !== null) return active;
  }
  return null;
}

function preferredActiveWindow(node: LayoutNode, focusedWindowId: WindowId | null): WindowId {
  const entries = descendantEntries(node);
  const focused = entries.find(entry => entry.windowId === focusedWindowId);
  if (focused !== undefined) return focused.windowId;
  const explicitActive = explicitActiveWindow(node);
  if (explicitActive !== null) return explicitActive;
  return entries[0]?.windowId ?? "";
}

function normalizedWeights(node: LayoutSplitNode): number[] {
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

function projectStack(
  node: LayoutNode,
  rect: PixelRect,
  context: NodeProjectionContext,
  kind: ProjectedStack["kind"],
  parentAxis: LayoutAxis | null
): void {
  const entries = descendantEntries(node, parentAxis);
  if (entries.length === 0) return;
  const activeWindowId = preferredActiveWindow(node, context.focusedWindowId);
  context.collector.stacks.push({
    nodeId: node.id,
    groupId: context.groupId,
    kind,
    windowIds: entries.map(entry => entry.windowId),
    activeWindowId,
    rect,
  });
  for (const entry of entries) {
    context.collector.windows.push({
      windowId: entry.windowId,
      nodeId: entry.nodeId,
      groupId: context.groupId,
      rect,
      stackId: node.id,
      active: entry.windowId === activeWindowId,
      adapted: kind === "adaptive",
      parentAxis: entry.parentAxis,
    });
  }
}

function seamForBoundary(
  node: LayoutSplitNode,
  childRects: readonly PixelRect[],
  boundaryIndex: number,
  context: NodeProjectionContext
): ProjectedSeam | null {
  const leadingNode = node.children[boundaryIndex];
  const trailingNode = node.children[boundaryIndex + 1];
  const leading = childRects[boundaryIndex];
  const trailing = childRects[boundaryIndex + 1];
  if (
    leadingNode === undefined ||
    trailingNode === undefined ||
    leading === undefined ||
    trailing === undefined
  )
    return null;
  const leadingMinimum = minimumForNode(leadingNode, context.innerGap, context.minimums);
  const trailingMinimum = minimumForNode(trailingNode, context.innerGap, context.minimums);
  const weights = normalizedWeights(node);
  const horizontal = node.axis === "horizontal";
  const axisSpan = childRects.reduce(
    (sum, childRect) => sum + (horizontal ? childRect.w : childRect.h),
    0
  );
  const shared = {
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
      id: `${node.id}:${boundaryIndex}`,
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
    id: `${node.id}:${boundaryIndex}`,
    orientation: "horizontal",
    rect: { x: leading.x, y: start, w: leading.w, h: height },
    value: start + height / 2,
    minValue: leading.y + leadingMinimum.height + height / 2,
    maxValue: trailing.y + trailing.h - trailingMinimum.height - height / 2,
  };
}

function splitAllocationsMeetMinimums(
  node: Extract<LayoutNode, { kind: "split" }>,
  childRects: readonly PixelRect[],
  context: NodeProjectionContext
): boolean {
  return (
    childRects.length === node.children.length &&
    node.children.every((child, index) => {
      const childRect = childRects[index];
      if (childRect === undefined) return false;
      const childMinimum = minimumForNode(child, context.innerGap, context.minimums);
      return childRect.w >= childMinimum.width && childRect.h >= childMinimum.height;
    })
  );
}

function projectNode(
  node: LayoutNode,
  rect: PixelRect,
  context: NodeProjectionContext,
  parentAxis: LayoutAxis | null
): void {
  if (node.kind === "leaf") {
    const minimum = minimumForWindow(node.windowId, context.minimums);
    if (rect.w < minimum.width || rect.h < minimum.height) {
      context.collector.diagnostics.push({
        code: "minimum-unmet",
        nodeId: node.id,
        windowId: node.windowId,
        required: minimum,
        available: { width: rect.w, height: rect.h },
      });
    }
    context.collector.windows.push({
      windowId: node.windowId,
      nodeId: node.id,
      groupId: context.groupId,
      rect,
      stackId: null,
      active: true,
      adapted: false,
      parentAxis,
    });
    return;
  }
  if (node.kind === "stack") {
    projectStack(node, rect, context, "explicit", parentAxis);
    return;
  }
  const required = minimumForNode(node, context.innerGap, context.minimums);
  const childRects = splitRects(node, rect, context.innerGap);
  if (
    rect.w < required.width ||
    rect.h < required.height ||
    !splitAllocationsMeetMinimums(node, childRects, context)
  ) {
    context.collector.diagnostics.push({
      code: "adaptive-stack",
      nodeId: node.id,
      required,
      available: { width: rect.w, height: rect.h },
    });
    projectStack(node, rect, context, "adaptive", parentAxis);
    return;
  }
  node.children.forEach((child, index) => {
    const childRect = childRects[index];
    if (childRect !== undefined) projectNode(child, childRect, context, node.axis);
  });
  for (let index = 0; index < node.children.length - 1; index += 1) {
    const seam = seamForBoundary(node, childRects, index, context);
    if (seam !== null) context.collector.seams.push(seam);
  }
}

/** Projects normalized daemon topology without mutating it or persisting viewport decisions. */
export function projectLayout(input: LayoutProjectionInput): LayoutProjection {
  const workArea = pixelRect(input.workArea);
  const layoutArea = windowManagerLayoutArea(workArea, input.gaps);
  const collector: ProjectionCollector = { windows: [], stacks: [], seams: [], diagnostics: [] };
  const minimums = input.minimums ?? {};
  const innerGap = nonNegativeInteger(input.gaps.inner);
  for (const group of input.desktop.groups) {
    if (!isValidNormalizedRect(group.frame)) {
      collector.diagnostics.push({ code: "invalid-group-frame", groupId: group.id });
    }
    const rect = projectGroupFrame(group.frame, layoutArea);
    projectNode(
      group.root,
      rect,
      {
        groupId: group.id,
        innerGap,
        minimums,
        focusedWindowId: input.focusedWindowId ?? null,
        collector,
      },
      null
    );
  }
  return {
    revision: input.revision,
    desktopId: input.desktop.id,
    workArea,
    windows: collector.windows,
    stacks: collector.stacks,
    seams: collector.seams,
    diagnostics: collector.diagnostics,
  };
}

/** Keeps a floating commit contained while preserving the title-bar grab relationship. */
export function clampFloatingRect(input: FloatingRectCommit): PixelRect {
  const area = pixelRect(input.workArea);
  const proposed = pixelRect(input.proposedRect);
  const width = Math.min(proposed.w, area.w);
  const height = Math.min(proposed.h, area.h);
  const titleBarHeight = Math.min(height, nonNegativeInteger(input.titleBarHeight ?? 32));
  const grabOffset = input.grabOffset ?? { x: 0, y: 0 };
  const grabX = Math.min(width, Math.max(0, finite(grabOffset.x)));
  const grabY = Math.min(titleBarHeight, Math.max(0, finite(grabOffset.y)));
  const desiredX = input.pointer === undefined ? proposed.x : finite(input.pointer.x) - grabX;
  const desiredY = input.pointer === undefined ? proposed.y : finite(input.pointer.y) - grabY;
  const maximumX = area.x + area.w - width;
  const maximumY = area.y + area.h - height;
  return {
    x: Math.min(maximumX, Math.max(area.x, Math.round(desiredX))),
    y: Math.min(maximumY, Math.max(area.y, Math.round(desiredY))),
    w: width,
    h: height,
  };
}
