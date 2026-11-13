import type * as ShortcutTypes from "./window-manager-shortcut-types";

/** Stable identifiers from the daemon-authoritative window-manager snapshot. */
export type LayoutRevision = number;
export type DesktopId = string;
export type TabId = string;
export type LayoutNodeId = string;
export type WindowId = string;
export type WindowManagerClientId = string;
export type WindowManagerClientKind = "shell" | "browser";
/** `idle`: no stream attempted for this binding yet — there is nothing to report. */
export type WindowManagerConnectionStatus =
  | "idle"
  | "disconnected"
  | "connecting"
  | "connected"
  | "reconnecting";

export type LayoutAxis = "horizontal" | "vertical";
/**
 * `window.move` placements: an edge splits beside a target window (or at a tab's root edge),
 * `tab` makes the window a tab of its own.
 */
export type DropPlacement = "tab" | "left" | "right" | "top" | "bottom";
export type FocusDirection = "left" | "right" | "up" | "down";
export type WindowManagerNewWindowPolicy = "beside_focus" | "tab";
export type WindowManagerSmallViewportPolicy = "stack" | "reject";
export type WindowManagerFocusPolicy = "click_directional" | "directional";
export type WindowManagerDesktopTransition = "slide" | "crossfade" | "instant";
export type WindowManagerDragModifier = "alt" | "control" | "meta" | "shift" | "none";

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

/** A tab's content: one window, or a weighted split tree whose every leaf is a window. */
export type LayoutNode = LayoutLeafNode | LayoutSplitNode;

/** One entry of a desktop's tab bar. Pinned tabs form a contiguous prefix of the bar. */
export interface LayoutTab {
  id: TabId;
  pinned: boolean;
  root: LayoutNode;
}

export interface LayoutDesktop {
  id: DesktopId;
  name: string;
  order: number;
  tabs: readonly LayoutTab[];
  /** Durable default for clients with no active tab of their own on this desktop. */
  activeTabId: TabId;
}

export interface WindowManagerRoute {
  pathname: string;
  search: Readonly<Record<string, unknown>>;
}

export interface WindowManagerWindow {
  id: WindowId;
  app: string;
  instanceKey: string | null;
  route: WindowManagerRoute;
  navStack: readonly WindowManagerRoute[];
  desktopId: DesktopId;
  tabId: TabId;
}

export interface WindowManagerMigrationSummary {
  desktops: number;
  tabs: number;
  windows: number;
  floatingToTabs: number;
  minimizedToTabs: number;
  islandsMerged: number;
  pinsDropped: number;
  zoomDesktopsMerged: number;
}

/** Present on the snapshot while an upgrade notice is unacknowledged. */
export interface WindowManagerMigrationStatus {
  state: string;
  fromVersion: number | null;
  at: string | null;
  acknowledged: boolean;
  summary: WindowManagerMigrationSummary | null;
  error: WindowManagerDiagnosticPayload | null;
  failureRevision: LayoutRevision | null;
}

/** A retired surface a command used; reported once per surface. */
export interface WindowManagerDeprecation {
  code: string;
  surface: string;
  replacement: string | null;
  removalRelease: string;
  message: string;
}

export interface WindowManagerGapsConfig {
  inner: number;
  top: number;
  right: number;
  bottom: number;
  left: number;
}

export interface WindowManagerWorkspaceConfig {
  newWindowPolicy?: WindowManagerNewWindowPolicy;
  smallViewportPolicy?: WindowManagerSmallViewportPolicy;
  focusPolicy?: WindowManagerFocusPolicy;
  focusWrap?: boolean;
  focusFollowsPointer?: boolean;
  swapModifier?: WindowManagerDragModifier;
  historyLimit?: number;
  navStackLimit?: number;
  closedEntryLimit?: number;
  desktopTransition?: WindowManagerDesktopTransition;
  gaps?: WindowManagerGapsConfig;
  shortcuts?: ShortcutTypes.WindowManagerShortcutMap;
  globalShortcuts?: ShortcutTypes.WindowManagerGlobalShortcutMap;
}

/** Effective global config before workspace-scoped overrides are applied. */
export interface WindowManagerConfig {
  newWindowPolicy: WindowManagerNewWindowPolicy;
  smallViewportPolicy: WindowManagerSmallViewportPolicy;
  focusPolicy: WindowManagerFocusPolicy;
  focusWrap: boolean;
  focusFollowsPointer: boolean;
  swapModifier: WindowManagerDragModifier;
  historyLimit: number;
  navStackLimit: number;
  closedEntryLimit: number;
  desktopTransition: WindowManagerDesktopTransition;
  gaps: WindowManagerGapsConfig;
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
  tabIds: readonly TabId[];
  nodeIds: readonly LayoutNodeId[];
  clientIds: readonly WindowManagerClientId[];
  /** Tabs that went from one member to several. */
  tabSplit: readonly TabId[];
  /** Tabs that went from several members to one, or dissolved. */
  tabUnsplit: readonly TabId[];
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
  version: 6;
  workspaceId: string;
  revision: LayoutRevision;
  desktops: readonly LayoutDesktop[];
  windows: Readonly<Record<WindowId, WindowManagerWindow>>;
  closedEntryCount: number;
  overrides: WindowManagerWorkspaceConfig;
  migration: WindowManagerMigrationStatus | null;
  updatedAt: string;
}

export interface WindowManagerClientView {
  workspaceId: string;
  clientId: WindowManagerClientId;
  presentationRevision: LayoutRevision;
  activeDesktopId: DesktopId;
  focusedWindowId: WindowId | null;
  focusOrder: readonly WindowId[];
  /** This client's active tab per desktop; repaired against the snapshot on read. */
  activeTab: Readonly<Record<DesktopId, TabId>>;
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
  /** The focused window shares its tab with other windows. */
  windowInSplit: boolean;
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

/** Commands this client issues. */
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
  | "window.pin"
  | "window.reopen"
  | "window.tab.reorder"
  | "window.tab.activate"
  | "layout.arrange"
  | "layout.resize"
  | "layout.balance"
  | "layout.undo"
  | "layout.redo"
  | "layout.replace";

/**
 * Command IDs an event can carry. Other actors may still use the retired IDs the daemon accepts
 * through v0.7.0 (`window.stack.group` commits as tab placement).
 */
export type WindowManagerEventCommandId =
  | WindowManagerCommandId
  | "window.resize"
  | "window.toggle_floating"
  | "window.zoom"
  | "window.stack.group"
  | "window.stack.reorder"
  | "window.stack.set_active"
  | "layout.frame_resize";

export interface WindowManagerRebaseGuard {
  windowId?: WindowId;
  tabId?: TabId;
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
  /**
   * When the input that authored the command was made (`Date.now()`); its deadline counts from
   * here instead of from dispatch. Local to this client: never part of the command on the wire.
   */
  inputAt?: number;
  /**
   * A `window.focus` whose caller does nothing with its result but show the window: a newer focus
   * may drop it before it is sent or resolve it `false` as replaced (never as refused). Local to
   * this client: never part of the command on the wire.
   */
  replaceable?: boolean;
}

/** How a caller dispatches `window.focus`; see `WindowManagerCommandInput.replaceable`. */
export interface FocusWindowOptions {
  replaceable?: boolean;
}

export interface WindowManagerCommandResult {
  snapshot: WindowManagerSnapshot;
  applied: boolean;
  changes: WindowManagerChangeSet;
  diagnostics: readonly WindowManagerDiagnosticPayload[];
  deprecations: readonly WindowManagerDeprecation[];
  client: WindowManagerClientView | null;
  rebasedFrom: LayoutRevision | null;
}

export interface WindowManagerEvent {
  workspaceId: string;
  revision: LayoutRevision;
  commandId: WindowManagerEventCommandId;
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
  TabProjectionInput,
  ProjectedWindow,
  ProjectedSeam,
  CollapsedSplit,
  TabProjectionDiagnostic,
  TabProjection,
} from "./tab-projection-types";
