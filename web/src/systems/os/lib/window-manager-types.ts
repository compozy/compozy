/** Stable identifiers from the daemon-authoritative window-manager snapshot. */
export type LayoutRevision = number;
export type DesktopId = string;
export type GroupId = string;
export type LayoutNodeId = string;
export type WindowId = string;
export type WindowManagerClientId = string;
export type WindowManagerClientKind = "shell" | "browser";
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

export interface LayoutFloatingStack {
  id: LayoutNodeId;
  windowIds: readonly WindowId[];
  activeId: WindowId | null;
  rect: NormalizedRect;
  minimized: boolean;
}

export interface LayoutDesktop {
  id: DesktopId;
  name: string;
  order: number;
  purpose: DesktopPurpose;
  focusOwner: WindowId | null;
  groups: readonly LayoutGroup[];
  floating: readonly WindowId[];
  floatingStacks: readonly LayoutFloatingStack[];
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
  navStack: readonly {
    pathname: string;
    search: Readonly<Record<string, unknown>>;
  }[];
  pinned: boolean;
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

export type WindowManagerShortcutBinding = readonly string[];
export type WindowManagerShortcutMap = Readonly<Record<string, WindowManagerShortcutBinding>>;

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
  navStackLimit?: number;
  closedEntryLimit?: number;
  desktopTransition?: WindowManagerDesktopTransition;
  gaps?: WindowManagerGapsConfig;
  snap?: WindowManagerSnapConfig;
  bindings?: WindowManagerBindingsConfig;
  shortcuts?: WindowManagerShortcutMap;
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
  navStackLimit: number;
  closedEntryLimit: number;
  desktopTransition: WindowManagerDesktopTransition;
  gaps: WindowManagerGapsConfig;
  snap: WindowManagerSnapConfig;
  bindings: WindowManagerBindingsConfig;
  /** Stored operator overrides. Empty bindings disable an action. */
  shortcuts: WindowManagerShortcutMap;
  /** Daemon-owned defaults, served with the settings contract. */
  shortcutDefaults: WindowManagerShortcutMap;
  /** Full, validated defaults + overrides map used by the live shell. */
  effectiveShortcuts: WindowManagerShortcutMap;
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
  stackGrouped: readonly LayoutNodeId[];
  stackUngrouped: readonly LayoutNodeId[];
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

export interface WindowManagerSnapshot {
  version: 3;
  workspaceId: string;
  revision: LayoutRevision;
  desktops: readonly LayoutDesktop[];
  windows: Readonly<Record<WindowId, WindowManagerWindow>>;
  closedEntryCount: number;
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
  stackActive: Readonly<Record<LayoutNodeId, WindowId>>;
  connectedAt: string;
}

/** Full daemon attachment projection; presentation-only consumers use the base view above. */
export interface WindowManagerAttachedClientView extends WindowManagerClientView {
  kind: WindowManagerClientKind;
  contextRevision: LayoutRevision;
  paletteContext: WindowManagerPaletteContext;
  /** Present only on the registration response; list and stream projections omit it. */
  attachmentToken: string | null;
}

export interface WindowManagerPaletteContext {
  windowFocused: boolean;
  windowFloating: boolean;
  windowStacked: boolean;
  desktopWindowCount: number;
  scopeGlobal: boolean;
  shellDesktop: boolean;
  focusedSessionState: string | null;
  workspaceTrusted: boolean;
  destinationIntent: {
    pathname: string;
    search: Readonly<Record<string, unknown>>;
  } | null;
}

export interface WindowManagerClientCommand {
  workspaceId: string;
  commandId: string;
  op: string;
  payload: unknown;
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
  | "window.resize"
  | "window.swap"
  | "window.toggle_floating"
  | "window.zoom"
  | "window.stack.group"
  | "window.stack.reorder"
  | "window.stack.set_active"
  | "window.pin"
  | "window.reopen"
  | "layout.arrange"
  | "layout.resize"
  | "layout.frame_resize"
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
      client: WindowManagerAttachedClientView | null;
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
      client: WindowManagerAttachedClientView;
    }
  | { type: "client_command"; command: WindowManagerClientCommand }
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
