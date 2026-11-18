/** Viewport projection contracts for one tab's split tree (pure, per client). */
import type {
  LayoutAxis,
  LayoutNodeId,
  LayoutTab,
  PixelRect,
  PixelSize,
  TabId,
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

export interface TabProjectionInput {
  tab: LayoutTab;
  /** The work area the tab's content fills, before the outer gaps. */
  workArea: PixelRect;
  gaps: ProjectionGaps;
  minimums?: WindowMinimums;
  /** This client's focus history, most recent first: picks the member a collapsed split shows. */
  focusOrder?: readonly WindowId[];
  /** Compact viewport: the root split shows one member at a time as well. */
  compact?: boolean;
}

export interface ProjectedWindow {
  windowId: WindowId;
  nodeId: LayoutNodeId;
  rect: PixelRect;
  /** False for the members a collapsed split keeps behind its switcher. */
  shown: boolean;
  parentAxis: LayoutAxis | null;
}

/**
 * A split whose members cannot fit at their pixel minimums (or any split in the compact
 * viewport): it shows one member at a time with a switcher. Only that node collapses; the
 * snapshot's weights are untouched.
 */
export interface CollapsedSplit {
  splitId: LayoutNodeId;
  windowIds: readonly WindowId[];
  shownWindowId: WindowId;
  rect: PixelRect;
}

export interface ProjectedSeam {
  /** Stable structural identity used by `layout.resize`. */
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

export type TabProjectionDiagnostic =
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
    };

export interface TabProjection {
  tabId: TabId;
  workArea: PixelRect;
  windows: readonly ProjectedWindow[];
  seams: readonly ProjectedSeam[];
  collapsed: readonly CollapsedSplit[];
  diagnostics: readonly TabProjectionDiagnostic[];
}
