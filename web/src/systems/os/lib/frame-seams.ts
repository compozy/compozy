import { minimumForNode } from "./layout-minimums";
import type {
  DesktopId,
  GroupId,
  LayoutDesktop,
  LayoutGroup,
  LayoutNode,
  NormalizedRect,
  PixelRect,
  ProjectedFrameSeam,
  WindowId,
  WindowMinimums,
} from "./window-manager-types";

/** Abutment tolerance for island edges sharing one boundary line. */
const FRAME_EDGE_EPSILON = 0.0001;
/** Matches the daemon's weight tolerance no-op band. */
const FRAME_SEAM_EPSILON = 0.000001;
/** Daemon floor for island frame extents (`minGroupFrameExtent`). */
const MIN_FRAME_EXTENT = 0.01;

export interface FrameSeamsInput {
  desktopId: DesktopId;
  groups: readonly LayoutGroup[];
  layoutArea: PixelRect;
  innerGap: number;
  minimums: WindowMinimums;
}

export interface GroupFrameEditInput {
  groupId: GroupId;
  frame: NormalizedRect;
}

interface FrameEdge {
  group: LayoutGroup;
  side: "leading" | "trailing";
  line: number;
  spanStart: number;
  spanEnd: number;
}

interface EdgeCluster {
  line: number;
  spanStart: number;
  spanEnd: number;
  leading: LayoutGroup[];
  trailing: LayoutGroup[];
}

function collectEdges(groups: readonly LayoutGroup[], orientation: "vertical" | "horizontal") {
  const edges: FrameEdge[] = [];
  for (const group of groups) {
    const frame = group.frame;
    if (orientation === "vertical") {
      edges.push(
        {
          group,
          side: "leading",
          line: frame.x + frame.w,
          spanStart: frame.y,
          spanEnd: frame.y + frame.h,
        },
        { group, side: "trailing", line: frame.x, spanStart: frame.y, spanEnd: frame.y + frame.h }
      );
    } else {
      edges.push(
        {
          group,
          side: "leading",
          line: frame.y + frame.h,
          spanStart: frame.x,
          spanEnd: frame.x + frame.w,
        },
        { group, side: "trailing", line: frame.y, spanStart: frame.x, spanEnd: frame.x + frame.w }
      );
    }
  }
  return edges.filter(edge => edge.line > FRAME_EDGE_EPSILON && edge.line < 1 - FRAME_EDGE_EPSILON);
}

/**
 * Merges edges on one shared line into connected clusters: overlapping spans
 * must move together or a frame would stop being a rectangle. Point-touching
 * corners stay independent.
 */
function clusterEdges(edges: FrameEdge[]): EdgeCluster[] {
  const byLine = new Map<number, FrameEdge[]>();
  const lines: number[] = [];
  for (const edge of edges) {
    const line = lines.find(candidate => Math.abs(candidate - edge.line) <= FRAME_EDGE_EPSILON);
    if (line === undefined) {
      lines.push(edge.line);
      byLine.set(edge.line, [edge]);
    } else {
      byLine.get(line)?.push(edge);
    }
  }
  const clusters: EdgeCluster[] = [];
  for (const line of lines) {
    const lineEdges = [...(byLine.get(line) ?? [])].sort((a, b) => a.spanStart - b.spanStart);
    let current: EdgeCluster | null = null;
    for (const edge of lineEdges) {
      if (current !== null && edge.spanStart < current.spanEnd - FRAME_EDGE_EPSILON) {
        current.spanEnd = Math.max(current.spanEnd, edge.spanEnd);
        current.spanStart = Math.min(current.spanStart, edge.spanStart);
        current[edge.side].push(edge.group);
        continue;
      }
      current = {
        line,
        spanStart: edge.spanStart,
        spanEnd: edge.spanEnd,
        leading: edge.side === "leading" ? [edge.group] : [],
        trailing: edge.side === "trailing" ? [edge.group] : [],
      };
      clusters.push(current);
    }
  }
  return clusters.filter(cluster => cluster.leading.length > 0 && cluster.trailing.length > 0);
}

function descendantWindows(node: LayoutNode): WindowId[] {
  if (node.kind === "leaf") return [node.windowId];
  if (node.kind === "stack") return [...node.windowIds];
  return node.children.flatMap(descendantWindows);
}

function seamFromCluster(
  cluster: EdgeCluster,
  orientation: "vertical" | "horizontal",
  input: FrameSeamsInput
): ProjectedFrameSeam {
  const { layoutArea: area, innerGap } = input;
  const vertical = orientation === "vertical";
  const axisSize = vertical ? area.w : area.h;
  const axisOrigin = vertical ? area.x : area.y;
  const crossSize = vertical ? area.h : area.w;
  const crossOrigin = vertical ? area.y : area.x;
  const linePx = axisOrigin + Math.round(axisSize * cluster.line);
  const gap = Math.max(0, Math.round(innerGap));
  const trailingInset = gap - Math.ceil(gap / 2);
  const spanStartPx = crossOrigin + Math.round(crossSize * cluster.spanStart);
  const spanEndPx = crossOrigin + Math.round(crossSize * cluster.spanEnd);
  const minPx = (group: LayoutGroup) => {
    const minimum = minimumForNode(group.root, gap, input.minimums);
    return vertical ? minimum.width : minimum.height;
  };
  const groupStart = (group: LayoutGroup) =>
    axisOrigin + Math.round(axisSize * (vertical ? group.frame.x : group.frame.y));
  const groupEnd = (group: LayoutGroup) =>
    axisOrigin +
    Math.round(
      axisSize * (vertical ? group.frame.x + group.frame.w : group.frame.y + group.frame.h)
    );
  const half = gap / 2;
  const minValue = Math.max(
    ...cluster.leading.map(group => groupStart(group) + minPx(group)),
    axisOrigin
  );
  const maxValue = Math.min(
    ...cluster.trailing.map(group => groupEnd(group) - minPx(group)),
    axisOrigin + axisSize
  );
  const strip: PixelRect = vertical
    ? { x: linePx - trailingInset, y: spanStartPx, w: gap, h: spanEndPx - spanStartPx }
    : { x: spanStartPx, y: linePx - trailingInset, w: spanEndPx - spanStartPx, h: gap };
  const sortedIds = (groups: LayoutGroup[]) => groups.map(group => group.id).sort();
  const leadingIds = sortedIds(cluster.leading);
  const trailingIds = sortedIds(cluster.trailing);
  return {
    // Participant-derived identity: stable while a live preview moves the line,
    // so the dragged seam element never remounts and keeps its pointer capture.
    id: `frame:${orientation}:${leadingIds.join("+")}~${trailingIds.join("+")}`,
    desktopId: input.desktopId,
    orientation: vertical ? "vertical" : "horizontal",
    line: cluster.line,
    rect: strip,
    value: linePx - trailingInset + half,
    minValue: minValue + half,
    maxValue: maxValue - half,
    axisSpan: axisSize,
    leadingGroupIds: leadingIds,
    trailingGroupIds: trailingIds,
    leadingWindowIds: cluster.leading.flatMap(group => descendantWindows(group.root)),
    trailingWindowIds: cluster.trailing.flatMap(group => descendantWindows(group.root)),
  };
}

/** Projects every draggable shared boundary between abutting island frames. */
export function projectFrameSeams(input: FrameSeamsInput): ProjectedFrameSeam[] {
  if (input.groups.length < 2) return [];
  const seams: ProjectedFrameSeam[] = [];
  for (const orientation of ["vertical", "horizontal"] as const) {
    for (const cluster of clusterEdges(collectEdges(input.groups, orientation))) {
      seams.push(seamFromCluster(cluster, orientation, input));
    }
  }
  return seams;
}

export function frameSeamDeltaNormalized(seam: ProjectedFrameSeam, deltaPx: number): number {
  if (!Number.isFinite(deltaPx) || seam.axisSpan <= 0 || seam.minValue >= seam.maxValue) {
    return 0;
  }
  const clampedPx = Math.min(
    seam.maxValue - seam.value,
    Math.max(seam.minValue - seam.value, deltaPx)
  );
  return clampedPx / seam.axisSpan;
}

/**
 * Rewrites every participating island frame against the moved shared line.
 * All edited edges land on exactly one coordinate, healing float residue.
 */
export function frameSeamEdits(
  groups: readonly LayoutGroup[],
  seam: ProjectedFrameSeam,
  deltaPx: number
): GroupFrameEditInput[] {
  const delta = frameSeamDeltaNormalized(seam, deltaPx);
  if (Math.abs(delta) <= FRAME_SEAM_EPSILON) return [];
  const line = seam.line + delta;
  const vertical = seam.orientation === "vertical";
  const leadingIds = new Set(seam.leadingGroupIds);
  const trailingIds = new Set(seam.trailingGroupIds);
  const edits: GroupFrameEditInput[] = [];
  for (const group of groups) {
    const frame = group.frame;
    if (leadingIds.has(group.id)) {
      edits.push({
        groupId: group.id,
        frame: vertical ? { ...frame, w: line - frame.x } : { ...frame, h: line - frame.y },
      });
    } else if (trailingIds.has(group.id)) {
      edits.push({
        groupId: group.id,
        frame: vertical
          ? { ...frame, x: line, w: frame.x + frame.w - line }
          : { ...frame, y: line, h: frame.y + frame.h - line },
      });
    }
  }
  const valid = edits.every(
    edit => edit.frame.w >= MIN_FRAME_EXTENT && edit.frame.h >= MIN_FRAME_EXTENT
  );
  return valid ? edits : [];
}

/** Returns the desktop with the previewed frame-seam delta applied to its islands. */
export function applyFrameSeamPreviewToDesktop(
  desktop: LayoutDesktop,
  seam: ProjectedFrameSeam,
  deltaPx: number
): LayoutDesktop {
  const edits = frameSeamEdits(desktop.groups, seam, deltaPx);
  if (edits.length === 0) return desktop;
  const frames = new Map(edits.map(edit => [edit.groupId, edit.frame]));
  return {
    ...desktop,
    groups: desktop.groups.map(group => {
      const frame = frames.get(group.id);
      return frame === undefined ? group : { ...group, frame };
    }),
  };
}
