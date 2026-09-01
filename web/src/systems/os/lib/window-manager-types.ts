import type * as ShortcutTypes from "./window-manager-shortcut-types";

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
  /** The window was zoomed when it minimized; restore brings the zoom back. */
  zoomed: boolean;
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
  /** Fills the desktop work area with the unit holding this window. */
  zoomed: boolean;
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
  navStackLimit?: number;
  closedEntryLimit?: number;
  desktopTransition?: WindowManagerDesktopTransition;
  gaps?: WindowManagerGapsConfig;
  snap?: WindowManagerSnapConfig;
  bindings?: WindowManagerBindingsConfig;
  shortcuts?: ShortcutTypes.WindowManagerShortcutMap;
  globalShortcuts?: ShortcutTypes.WindowManagerGlobalShortcutMap;
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
  shortcuts: ShortcutTypes.WindowManagerShortcutMap;
  /** Daemon-owned defaults, served with the settings contract. */
  shortcutDefaults: ShortcutTypes.WindowManagerShortcutMap;
  /** Full, validated defaults + overrides map used by the live shell. */
  effectiveShortcuts: ShortcutTypes.WindowManagerShortcutMap;
  /** Daemon-owned intended desktop-global bindings. */
  globalShortcuts: ShortcutTypes.WindowManagerGlobalShortcutMap;
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
  version: 4;
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

/** Stream and list projection. Registration adds a required attachment token. */
export interface WindowManagerAttachedClientView extends WindowManagerClientView {
  kind: WindowManagerClientKind;
  contextRevision: LayoutRevision;
  paletteContext: WindowManagerPaletteContext;
  globalShortcuts: readonly ShortcutTypes.WindowManagerGlobalShortcutRegistration[];
}

export interface WindowManagerRegisteredClientView extends WindowManagerAttachedClientView {
  attachmentToken: string;
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
  /** Revision the command was authored against; the cached revision is used when absent. */
  expectedRevision?: LayoutRevision;
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
  | { type: "client_command"; workspaceId: string; command: WindowManagerClientCommand }
  | { type: "heartbeat"; workspaceId: string; revision: LayoutRevision }
  | { type: "error"; error: WindowManagerErrorPayload };

/** Projection contracts live beside the projector; re-exported so callers keep one import site. */
export type {
  ProjectionGaps,
  WindowMinimums,
  LayoutProjectionInput,
  ProjectedWindow,
  ProjectedStack,
  ProjectedSeam,
  ProjectedFrameSeam,
  LayoutProjectionDiagnostic,
  LayoutProjection,
} from "./layout-projection-types";
