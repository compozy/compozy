/** Stable identifiers from the daemon-authoritative window-manager snapshot. */
export type LayoutRevision = number;
export type DesktopId = string;
export type GroupId = string;
export type LayoutNodeId = string;
export type WindowId = string;
export type WindowManagerClientId = string;
export type WindowManagerConnectionStatus =
  | "disconnected"
  | "connecting"
  | "connected"
  | "reconnecting";

export type LayoutAxis = "horizontal" | "vertical";
export type DesktopPurpose = "standard" | "focus";
export type WindowPlacement = "tiled" | "stacked" | "floating";
export type DropPlacement =
  | "floating"
  | "before"
  | "after"
  | "left"
  | "right"
  | "top"
  | "bottom"
  | "center";
export type FocusDirection = "left" | "right" | "up" | "down";
export type WindowManagerEdgeCenterBinding = "none" | "reserved" | "zoom";
export type WindowManagerSmallViewportPolicy = "stack" | "reject";
export type WindowManagerFocusPolicy = "click_directional" | "directional";
export type WindowManagerDesktopTransition = "slide" | "crossfade" | "instant";
export type WindowManagerDragModifier = "alt" | "control" | "meta" | "shift" | "none";

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
  id: LayoutNodeId;
  kind: "leaf";
  windowId: WindowId;
}

export interface LayoutSplitNode {
  id: LayoutNodeId;
  kind: "split";
  axis: LayoutAxis;
  children: readonly LayoutNode[];
  weights: readonly number[];
}

export interface LayoutStackNode {
  id: LayoutNodeId;
  kind: "stack";
  windowIds: readonly WindowId[];
  activeId: WindowId;
}

/** Browser mirror of the daemon's normalized, discriminated layout topology. */
export type LayoutNode = LayoutLeafNode | LayoutSplitNode | LayoutStackNode;

export interface LayoutGroup {
  id: GroupId;
  frame: NormalizedRect;
  root: LayoutNode;
}

export interface LayoutDesktop {
  id: DesktopId;
  name: string;
  order: number;
  purpose: DesktopPurpose;
  focusOwner: WindowId | null;
  groups: readonly LayoutGroup[];
  floating: readonly WindowId[];
}

export interface WindowManagerReturnAnchor {
  desktopId: DesktopId;
  groupId: GroupId | null;
  parentSplitId: LayoutNodeId | null;
  childIndex: number | null;
  weight: number | null;
  neighborIds: readonly WindowId[];
  sourceRevision: LayoutRevision;
  sourceGroup: LayoutGroup | null;
}

export interface WindowManagerWindow {
  id: WindowId;
  app: string;
  instanceKey: string | null;
  route: {
    pathname: string;
    search: Readonly<Record<string, unknown>>;
  };
  placement: WindowPlacement;
  desktopId: DesktopId;
  floatingRect: NormalizedRect;
  minimized: boolean;
  returnAnchor: WindowManagerReturnAnchor | null;
}

export interface WindowManagerGapsConfig {
  inner: number;
  top: number;
  right: number;
  bottom: number;
  left: number;
}

export interface WindowManagerSnapConfig {
  edgeBand: number;
  cornerReach: number;
  exitSlack: number;
  repeatRatios: readonly number[];
}

export interface WindowManagerBindingsConfig {
  topCenter: WindowManagerEdgeCenterBinding;
  bottomCenter: WindowManagerEdgeCenterBinding;
}

export interface WindowManagerWorkspaceConfig {
  newWindowPolicy?: "floating" | "beside_focus";
  smallViewportPolicy?: WindowManagerSmallViewportPolicy;
  focusPolicy?: WindowManagerFocusPolicy;
  focusWrap?: boolean;
  focusFollowsPointer?: boolean;
  raiseOnFocus?: boolean;
  dragAwayPolicy?: "window" | "group";
  groupMoveModifier?: WindowManagerDragModifier;
  swapModifier?: WindowManagerDragModifier;
  historyLimit?: number;
  desktopTransition?: WindowManagerDesktopTransition;
  gaps?: WindowManagerGapsConfig;
  snap?: WindowManagerSnapConfig;
  bindings?: WindowManagerBindingsConfig;
  shortcuts?: Readonly<Record<string, string>>;
}

/** Effective global config before workspace-scoped overrides are applied. */
export interface WindowManagerConfig {
  newWindowPolicy: "floating" | "beside_focus";
  smallViewportPolicy: WindowManagerSmallViewportPolicy;
  focusPolicy: WindowManagerFocusPolicy;
  focusWrap: boolean;
  focusFollowsPointer: boolean;
  raiseOnFocus: boolean;
  dragAwayPolicy: "window" | "group";
  groupMoveModifier: WindowManagerDragModifier;
  swapModifier: WindowManagerDragModifier;
  historyLimit: number;
  desktopTransition: WindowManagerDesktopTransition;
  gaps: WindowManagerGapsConfig;
  snap: WindowManagerSnapConfig;
  bindings: WindowManagerBindingsConfig;
  shortcuts: Readonly<Record<string, string>>;
}

export interface WindowManagerActor {
  kind: string;
  id: string;
}

export interface WindowManagerChangeSet {
  desktopIds: readonly DesktopId[];
  windowIds: readonly WindowId[];
  groupIds: readonly GroupId[];
  nodeIds: readonly LayoutNodeId[];
  clientIds: readonly WindowManagerClientId[];
}

export interface WindowManagerDiagnosticPayload {
  code: string;
  path: string | null;
  message: string;
}

export interface WindowManagerConflictPayload {
  code: string;
  entityId: string | null;
  currentId: string | null;
}

export interface WindowManagerStateDocument {
  desktops: readonly LayoutDesktop[];
  windows: Readonly<Record<WindowId, WindowManagerWindow>>;
  overrides: WindowManagerWorkspaceConfig;
}

export interface WindowManagerHistoryEntry {
  commandId: WindowManagerCommandId;
  before: WindowManagerStateDocument;
  after: WindowManagerStateDocument;
  actor: WindowManagerActor;
  origin: string;
  createdAt: string;
}

export interface WindowManagerSnapshot {
  version: number;
  workspaceId: string;
  revision: LayoutRevision;
  desktops: readonly LayoutDesktop[];
  windows: Readonly<Record<WindowId, WindowManagerWindow>>;
  history: {
    undo: readonly WindowManagerHistoryEntry[];
    redo: readonly WindowManagerHistoryEntry[];
  };
  overrides: WindowManagerWorkspaceConfig;
  updatedAt: string;
}

export interface WindowManagerClientView {
  workspaceId: string;
  clientId: WindowManagerClientId;
  presentationRevision: LayoutRevision;
  activeDesktopId: DesktopId;
  focusedWindowId: WindowId | null;
  focusOrder: readonly WindowId[];
  connectedAt: string;
}

export type WindowManagerCommandId =
  | "desktop.create"
  | "desktop.update"
  | "desktop.reorder"
  | "desktop.switch"
  | "desktop.delete"
  | "window.open"
  | "window.navigate"
  | "window.close"
  | "window.focus"
  | "window.move"
  | "window.swap"
  | "window.toggle_floating"
  | "window.zoom"
  | "layout.arrange"
  | "layout.resize"
  | "layout.balance"
  | "layout.undo"
  | "layout.redo"
  | "layout.replace";

export interface WindowManagerRebaseGuard {
  windowId?: WindowId;
  sourceNodeId?: LayoutNodeId;
  targetNodeId?: LayoutNodeId;
  splitId?: LayoutNodeId;
  boundaryIndex?: number;
}

export interface WindowManagerCommandInput {
  commandId: WindowManagerCommandId;
  payload: Readonly<Record<string, unknown>>;
  rebase?: WindowManagerRebaseGuard;
}

export interface WindowManagerCommandResult {
  snapshot: WindowManagerSnapshot;
  applied: boolean;
  changes: WindowManagerChangeSet;
  diagnostics: readonly WindowManagerDiagnosticPayload[];
  client: WindowManagerClientView | null;
  rebasedFrom: LayoutRevision | null;
}

export interface WindowManagerEvent {
  workspaceId: string;
  revision: LayoutRevision;
  commandId: WindowManagerCommandId;
  changes: WindowManagerChangeSet;
  actor: WindowManagerActor;
  origin: string;
  occurredAt: string;
}

export interface WindowManagerErrorPayload {
  error: string;
  code: string;
  workspaceId: string;
  currentRevision: LayoutRevision | null;
  conflicts: readonly WindowManagerConflictPayload[];
  diagnostics: readonly WindowManagerDiagnosticPayload[];
}

export type WindowManagerStreamFrame =
  | {
      type: "snapshot";
      workspaceId: string;
      revision: LayoutRevision;
      snapshot: WindowManagerSnapshot;
      client: WindowManagerClientView | null;
    }
  | {
      type: "event";
      workspaceId: string;
      revision: LayoutRevision;
      event: WindowManagerEvent;
    }
  | {
      type: "client";
      workspaceId: string;
      revision: LayoutRevision;
      client: WindowManagerClientView;
    }
  | { type: "error"; error: WindowManagerErrorPayload };

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
}

export interface ProjectedWindow {
  windowId: WindowId;
  nodeId: LayoutNodeId;
  groupId: GroupId;
  rect: PixelRect;
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
  diagnostics: readonly LayoutProjectionDiagnostic[];
}
