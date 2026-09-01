/** Viewport projection contracts derived from the daemon topology (pure, per client). */
import type {
  DesktopId,
  GroupId,
  LayoutAxis,
  LayoutDesktop,
  LayoutNodeId,
  LayoutRevision,
  NormalizedRect,
  PixelRect,
  PixelSize,
  WindowId,
} from "./window-manager-types";

export interface ProjectionGaps {
  inner: number;
  top: number;
  right: number;
  bottom: number;
  left: number;
}

export type WindowMinimums = Readonly<Record<WindowId, PixelSize>>;

export interface LayoutProjectionInput {
  revision: LayoutRevision;
  desktop: LayoutDesktop;
  workArea: PixelRect;
  gaps: ProjectionGaps;
  minimums?: WindowMinimums;
  focusedWindowId?: WindowId | null;
  /** Per-client display-active per stack; overrides the durable `activeId` (ADR-009). */
  stackActive?: Readonly<Record<LayoutNodeId, WindowId>>;
}

export interface ProjectedWindow {
  windowId: WindowId;
  nodeId: LayoutNodeId;
  groupId: GroupId;
  rect: PixelRect;
  /** Normalized gap-free zone this unit owns inside the layout area. */
  zone: NormalizedRect;
  stackId: LayoutNodeId | null;
  active: boolean;
  adapted: boolean;
  parentAxis: LayoutAxis | null;
}

export interface ProjectedStack {
  nodeId: LayoutNodeId;
  groupId: GroupId;
  kind: "explicit" | "adaptive";
  windowIds: readonly WindowId[];
  activeWindowId: WindowId;
  rect: PixelRect;
  /** Normalized gap-free zone this unit owns inside the layout area. */
  zone: NormalizedRect;
}

export interface ProjectedSeam {
  /** Stable structural identity used by layout.resize. */
  id: string;
  splitId: LayoutNodeId;
  boundaryIndex: number;
  orientation: "horizontal" | "vertical";
  rect: PixelRect;
  value: number;
  minValue: number;
  maxValue: number;
  /** Pixel length the split's weights map onto (axis length minus gaps). */
  axisSpan: number;
  /** Normalized weights of the two children adjacent to this boundary. */
  leadingWeight: number;
  trailingWeight: number;
  leadingWindowIds: readonly WindowId[];
  trailingWindowIds: readonly WindowId[];
}

/**
 * One draggable shared boundary between abutting island frames. Every group
 * whose edge sits on the shared line moves together so frames stay rectangles.
 */
export interface ProjectedFrameSeam {
  id: string;
  desktopId: DesktopId;
  orientation: "horizontal" | "vertical";
  /** Normalized line coordinate every edited frame edge sits on. */
  line: number;
  rect: PixelRect;
  value: number;
  minValue: number;
  maxValue: number;
  /** Pixel length of the layout area along the drag axis. */
  axisSpan: number;
  leadingGroupIds: readonly GroupId[];
  trailingGroupIds: readonly GroupId[];
  leadingWindowIds: readonly WindowId[];
  trailingWindowIds: readonly WindowId[];
}

export type LayoutProjectionDiagnostic =
  | {
      code: "adaptive-stack";
      nodeId: LayoutNodeId;
      required: PixelSize;
      available: PixelSize;
    }
  | {
      code: "minimum-unmet";
      nodeId: LayoutNodeId;
      windowId: WindowId;
      required: PixelSize;
      available: PixelSize;
    }
  | {
      code: "invalid-group-frame";
      groupId: GroupId;
    };

export interface LayoutProjection {
  revision: LayoutRevision;
  desktopId: DesktopId;
  workArea: PixelRect;
  windows: readonly ProjectedWindow[];
  stacks: readonly ProjectedStack[];
  seams: readonly ProjectedSeam[];
  frameSeams: readonly ProjectedFrameSeam[];
  diagnostics: readonly LayoutProjectionDiagnostic[];
}
