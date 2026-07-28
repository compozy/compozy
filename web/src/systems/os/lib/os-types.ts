import type {
  DesktopId,
  DropPlacement,
  FocusDirection,
  GroupId,
  LayoutAxis,
  LayoutDesktop,
  LayoutNodeId,
  LayoutProjection,
  WindowPlacement,
  WindowManagerClientView,
  WindowManagerConfig,
  WindowManagerConnectionStatus,
  WindowManagerSnapshot,
} from "./window-manager-types";
import type { SnapCorner, SnapSide, SnapTarget } from "./snap-targets";

/** The app surfaces rendered inside daemon-managed logical windows. */
export type OsAppId =
  | "dashboard"
  | "session"
  | "tasks"
  | "agents"
  | "network"
  | "loops"
  | "jobs"
  | "triggers"
  | "marketplace"
  | "bridges"
  | "knowledge"
  | "sandbox"
  | "vault"
  | "settings";

export interface OsRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface OsWindowRoute {
  pathname: string;
  search: Record<string, unknown>;
}

/** Client projection of one authoritative daemon window. */
export interface OsWindow {
  id: string;
  app: OsAppId;
  instanceKey: string | null;
  route: OsWindowRoute;
  desktopId: DesktopId;
  placement: WindowPlacement;
  rect: OsRect;
  layer: number;
  minimized: boolean;
  groupId: GroupId | null;
  nodeId: LayoutNodeId | null;
  stackId: LayoutNodeId | null;
  stackActive: boolean;
  parentAxis: LayoutAxis | null;
}

export type OsWallpaper = "ember" | "mesh" | "carbon";
export type OsPresentation = "floating" | "compact";
export type OsViewportState = "ready" | "rejected";
export type OsHydration = "pending" | "live" | "degraded";
export type OsArrangePreset = "two-up" | "grid";

export interface OsOpenTarget {
  app: OsAppId;
  instanceKey?: string;
  route?: OsWindowRoute;
}

export interface OsDesktopBounds {
  width: number;
  height: number;
  /** Viewport offset of the win-layer origin; converts client to layer coordinates. */
  origin: { x: number; y: number };
}

/** Pointer context for a floating drop, in layer coordinates. */
export interface OsFloatingDrop {
  pointer: { x: number; y: number };
  grabOffset: { x: number; y: number };
}

export interface MoveWindowInput {
  destinationDesktopId: DesktopId;
  targetWindowId?: string;
  placement: DropPlacement;
  floatingRect?: OsRect;
  moveGroup?: boolean;
}

export interface WindowManagerCommandOutcome {
  accepted: boolean;
  /** Total completion: diagnosed runtime failures resolve false instead of rejecting. */
  completion: Promise<boolean>;
}

export interface WindowManagerOpenOutcome extends WindowManagerCommandOutcome {
  windowId: string;
}

/**
 * Selector surface derived from Query snapshots plus client-local presentation.
 * It is never persisted and never owns a revision.
 */
export interface OsDesktopRuntimeStore {
  snapshot: WindowManagerSnapshot | null;
  windowManagerConfig: WindowManagerConfig | null;
  client: WindowManagerClientView | null;
  desktops: readonly LayoutDesktop[];
  projections: Readonly<Record<DesktopId, LayoutProjection>>;
  windows: Readonly<Record<string, OsWindow>>;
  activeDesktopId: DesktopId | null;
  focusedId: string | null;
  railCollapsedAgentIds: readonly string[];
  wallpaper: OsWallpaper;
  reduceMotion: boolean;
  dockMagnify: boolean;
  presentation: OsPresentation;
  viewportState: OsViewportState;
  hydration: OsHydration;
  connectionStatus: WindowManagerConnectionStatus;
  desktopBounds: OsDesktopBounds | null;
  openOrFocus(target: OsOpenTarget): WindowManagerOpenOutcome;
  closeWindow(id: string): Promise<boolean>;
  focusWindow(id: string): WindowManagerCommandOutcome;
  minimizeWindow(id: string): Promise<boolean>;
  restoreWindow(id: string): void;
  zoomWindow(id: string): WindowManagerCommandOutcome;
  toggleFloating(id: string): void;
  moveWindow(id: string, input: MoveWindowInput): void;
  arrangeLayout(anchorId: string, preset: OsArrangePreset): void;
  commitFloatingRect(id: string, rect: OsRect, drop?: OsFloatingDrop): WindowManagerCommandOutcome;
  resizeLayout(splitId: string, boundaryIndex: number, delta: number): WindowManagerCommandOutcome;
  balanceLayout(groupId?: string, splitId?: string): void;
  navigateWindow(id: string, route: OsWindowRoute): WindowManagerCommandOutcome;
  toggleRailGroup(agentId: string): void;
  setWallpaper(wallpaper: OsWallpaper): void;
  setDockMagnify(on: boolean): void;
  setReduceMotion(on: boolean): void;
  setDesktopBounds(bounds: OsDesktopBounds): void;
}

export interface OsDesktopRuntime {
  getState(): OsDesktopRuntimeStore;
  getInitialState(): OsDesktopRuntimeStore;
  subscribe(
    listener: (state: OsDesktopRuntimeStore, previous: OsDesktopRuntimeStore) => void
  ): () => void;
}

export interface WindowManagerController extends OsDesktopRuntime {
  bind(binding: { workspaceId: string; clientId: string }): void;
  unbind(): void;
  setClient(client: WindowManagerClientView | null): void;
  setConnectionStatus(status: WindowManagerConnectionStatus): void;
  setLoadError(error: Error | null): void;
  createDesktop(): void;
  renameDesktop(desktopId: string, name: string): void;
  reorderDesktop(desktopId: string, order: number): void;
  switchDesktop(desktopId: string): void;
  switchDesktopDirection(direction: "previous" | "next"): void;
  deleteDesktop(desktopId: string, destinationId: string | null): void;
  moveWindowToDesktop(windowId: string, destinationDesktopId: string): void;
  tileWindow(windowId: string, edge: SnapSide | SnapCorner): void;
  applySnapTarget(
    windowId: string,
    target: SnapTarget,
    moveGroup?: boolean
  ): WindowManagerCommandOutcome;
  focusDirection(direction: FocusDirection): void;
  undoLayout(): void;
  redoLayout(): void;
  balanceFocusedLayout(): void;
  clearConflict(): void;
  refreshSnapshot(): void;
}

export const OS_COMPACT_BREAKPOINT = 960;
export const OS_WINDOW_MIN_WIDTH = 280;
export const OS_WINDOW_MIN_HEIGHT = 180;

export function osWindowId(app: OsAppId, instanceKey?: string | null): string {
  return app === "session" && instanceKey ? `session:${instanceKey}` : `app:${app}`;
}
