/**
 * Frozen v5 projection for the layout editor; S7 replaces it.
 *
 * The shell moved to the v6 tab model, but the Settings layout editor still edits v5 layout
 * documents (groups, frames, stacks, floating windows). These are the v5 geometry contracts it
 * renders against, copied unchanged from the retired `systems/os` projection.
 */

export type LayoutAxis = "horizontal" | "vertical";

export interface NormalizedRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface PixelPoint {
  x: number;
  y: number;
}

export interface PixelRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface PixelSize {
  width: number;
  height: number;
}

export interface LayoutLeafNode {
  id: string;
  kind: "leaf";
  windowId: string;
}

export interface LayoutSplitNode {
  id: string;
  kind: "split";
  axis: LayoutAxis;
  children: readonly LayoutNode[];
  weights: readonly number[];
}

export interface LayoutStackNode {
  id: string;
  kind: "stack";
  windowIds: readonly string[];
  activeId: string;
}

export type LayoutNode = LayoutLeafNode | LayoutSplitNode | LayoutStackNode;

export interface LayoutGroup {
  id: string;
  frame: NormalizedRect;
  root: LayoutNode;
}

export interface LayoutFloatingStack {
  id: string;
  windowIds: readonly string[];
  activeId: string | null;
  rect: NormalizedRect;
  minimized: boolean;
}

export interface LayoutDesktop {
  id: string;
  name: string;
  order: number;
  groups: readonly LayoutGroup[];
  floating: readonly string[];
  floatingStacks: readonly LayoutFloatingStack[];
}

export interface ProjectionGaps {
  inner: number;
  top: number;
  right: number;
  bottom: number;
  left: number;
}

export type WindowMinimums = Readonly<Record<string, PixelSize>>;

export interface LayoutProjectionInput {
  revision: number;
  desktop: LayoutDesktop;
  workArea: PixelRect;
  gaps: ProjectionGaps;
  minimums?: WindowMinimums;
  focusedWindowId?: string | null;
  /** Per-client display-active per stack; overrides the durable `activeId`. */
  stackActive?: Readonly<Record<string, string>>;
}

export interface ProjectedWindow {
  windowId: string;
  nodeId: string;
  groupId: string;
  rect: PixelRect;
  /** Normalized gap-free zone this unit owns inside the layout area. */
  zone: NormalizedRect;
  stackId: string | null;
  active: boolean;
  adapted: boolean;
  parentAxis: LayoutAxis | null;
}

export interface ProjectedStack {
  nodeId: string;
  groupId: string;
  kind: "explicit" | "adaptive";
  windowIds: readonly string[];
  activeWindowId: string;
  rect: PixelRect;
  /** Normalized gap-free zone this unit owns inside the layout area. */
  zone: NormalizedRect;
}

export interface ProjectedSeam {
  /** Stable structural identity used by layout.resize. */
  id: string;
  splitId: string;
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
  leadingWindowIds: readonly string[];
  trailingWindowIds: readonly string[];
}

export type LayoutProjectionDiagnostic =
  | {
      code: "adaptive-stack";
      nodeId: string;
      required: PixelSize;
      available: PixelSize;
    }
  | {
      code: "minimum-unmet";
      nodeId: string;
      windowId: string;
      required: PixelSize;
      available: PixelSize;
    }
  | {
      code: "invalid-group-frame";
      groupId: string;
    };

export interface LayoutProjection {
  revision: number;
  desktopId: string;
  workArea: PixelRect;
  windows: readonly ProjectedWindow[];
  stacks: readonly ProjectedStack[];
  seams: readonly ProjectedSeam[];
  diagnostics: readonly LayoutProjectionDiagnostic[];
}
