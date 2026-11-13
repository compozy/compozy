import type {
  DesktopId,
  DropPlacement,
  FocusDirection,
  LayoutDesktop,
  LayoutNodeId,
  LayoutRevision,
  TabId,
  TabProjection,
  WindowManagerClientView,
  WindowManagerRegisteredClientView,
  WindowManagerConfig,
  WindowManagerConnectionStatus,
  WindowManagerRebaseGuard,
  WindowManagerSnapshot,
} from "./window-manager-types";

/** The app surfaces rendered inside daemon-managed logical windows. */
export type OsAppId =
  | "dashboard"
  | "new-tab"
  | "session"
  | "terminal"
  | "tasks"
  | "agents"
  | "files"
  | "loops"
  | "automations"
  | "marketplace"
  | "vault"
  | "usage"
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

/** What an app's loading fallback is told about the window it stands in for. */
export interface OsWindowFallbackProps {
  /** The route the window is about to render. */
  route: OsWindowRoute;
  /**
   * The window reads every project's data: unbound, on the All projects desktop. Apps that list
   * one project's items there and everything's here read it to pick the shape they load into.
   */
  aggregate: boolean;
}

/** Client projection of one authoritative daemon window. */
export interface OsWindow {
  id: string;
  app: OsAppId;
  instanceKey: string | null;
  route: OsWindowRoute;
  /** Durable navigation ancestors, oldest first; `route` stays the leaf (ADR-011). */
  navStack: readonly OsWindowRoute[];
  desktopId: DesktopId;
  tabId: TabId;
  /** The leaf node that holds the window in its tab's tree. */
  nodeId: LayoutNodeId | null;
  /** The window's tab is pinned. */
  pinned: boolean;
  /** The window shares its tab with other windows. */
  inSplit: boolean;
  /** Its tab is this client's active tab on its desktop. */
  tabActive: boolean;
  /**
   * On screen for this client once its desktop is active: its tab is active and no collapsed
   * split keeps it behind the member switcher.
   */
  shown: boolean;
}

export type OsWallpaper = "flat" | "ember" | "mesh" | "carbon";
/** `compact`: below the compact breakpoint every split shows one member at a time. */
export type OsPresentation = "split" | "compact";
export type OsViewportState = "ready" | "rejected";
export type OsHydration = "pending" | "live" | "degraded";
export type OsArrangePreset = "two-up" | "grid" | "main-stack" | "columns";

export interface OsOpenTarget {
  app: OsAppId;
  instanceKey?: string;
  route?: OsWindowRoute;
  /** Open as a new tab right after the tab holding this window. */
  afterWindowId?: string;
  /** Skip focus-first resolution and always create another instance (ADR-002). */
  forceNewInstance?: boolean;
  /**
   * Explicit placement for a window this open creates. Omitted follows the daemon's new-window
   * policy (a tab after the active tab by default); `split` opens beside the focused window.
   */
  placement?: "split";
  /** Open onto this desktop instead of the client's active one. */
  desktopId?: string;
  /** Classified navigation intent when the target resolves to a live window (ADR-011). */
  navigateMode?: OsNavigateMode;
}

/** Navigation intent, classified where it is known rather than inferred (ADR-011). */
export type OsNavigateMode = "replace" | "push" | "pop";

/**
 * `window.close` breadth: `tab` closes the one window, `group` every window of its tab, and the
 * rest are bar scopes over the desktop's unpinned tabs.
 */
export type OsCloseScope = "tab" | "group" | "others" | "right" | "left" | "all";

export interface OsDesktopBounds {
  /** Full shell width, before dock tracks consume the work area. */
  viewportWidth?: number;
  width: number;
  height: number;
  /** Viewport offset of the content area's origin. */
  origin: { x: number; y: number };
}

export interface MoveWindowInput {
  destinationDesktopId: DesktopId;
  /** Split beside this window at the placement's edge, or the tab goes right after its tab. */
  targetWindowId?: string;
  /** Edge placement with no target window: the window joins this tab at its root edge. */
  targetTabId?: string;
  placement: DropPlacement;
  /** Move the window's whole tab instead of the one window. */
  moveGroup?: boolean;
}

/**
 * Fences a gesture's command to the layout the operator saw: a stale command still applies when
 * the guarded window, node or tab is where the gesture found it, and is refused otherwise.
 */
export interface WindowManagerCommandFence {
  expectedRevision: LayoutRevision;
  rebase: WindowManagerRebaseGuard;
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
  /** Registration proof retained locally; never projected by daemon streams. */
  clientAttachmentToken: string | null;
  desktops: readonly LayoutDesktop[];
  /** This client's active tab per desktop, repaired against the snapshot. */
  activeTabIds: Readonly<Record<DesktopId, TabId>>;
  /** Pixel projection of every tab's split tree against the measured content area. */
  projections: Readonly<Record<TabId, TabProjection>>;
  windows: Readonly<Record<string, OsWindow>>;
  activeDesktopId: DesktopId | null;
  focusedId: string | null;
  wallpaper: OsWallpaper;
  reduceMotion: boolean;
  presentation: OsPresentation;
  viewportState: OsViewportState;
  hydration: OsHydration;
  loadError: Error | null;
  connectionStatus: WindowManagerConnectionStatus;
  desktopBounds: OsDesktopBounds | null;
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
  setClient(client: WindowManagerClientView | WindowManagerRegisteredClientView | null): void;
  setConnectionStatus(status: WindowManagerConnectionStatus): void;
  setLoadError(error: Error | null): void;
  openOrFocus(target: OsOpenTarget): WindowManagerOpenOutcome;
  closeWindow(id: string, viewOnly?: boolean): Promise<boolean>;
  focusWindow(id: string): WindowManagerCommandOutcome;
  moveWindow(
    id: string,
    input: MoveWindowInput,
    fence?: WindowManagerCommandFence
  ): WindowManagerCommandOutcome;
  /** Swaps two windows of one tab; both keep their places' shares. */
  swapWindows(
    firstWindowId: string,
    secondWindowId: string,
    fence?: WindowManagerCommandFence
  ): WindowManagerCommandOutcome;
  arrangeLayout(anchorId: string, preset: OsArrangePreset): void;
  /** "Move to new tab": a split member becomes its own tab right after its source tab. */
  unsplitWindow(windowId: string): void;
  /** Moves a split member to a root edge of its own tab. */
  tileWindow(windowId: string, edge: "left" | "right" | "top" | "bottom"): void;
  /** Joins `chosenTabId` to `hostTabId` as a split with the chosen tab on the right. */
  splitWithTab(hostTabId: TabId, chosenTabId: TabId): void;
  /** Moves the window's tab one step left or right within its pinned or unpinned range. */
  moveTab(windowId: string, direction: "left" | "right"): void;
  resizeLayout(splitId: string, boundaryIndex: number, delta: number): WindowManagerCommandOutcome;
  balanceLayout(target: { tabId?: string; splitId?: string }): void;
  navigateWindow(
    id: string,
    route: OsWindowRoute,
    mode?: OsNavigateMode
  ): WindowManagerCommandOutcome;
  retargetWindow(
    id: string,
    instanceKey: string,
    route: OsWindowRoute
  ): WindowManagerCommandOutcome;
  popWindowRoute(id: string, expectedRoute?: OsWindowRoute): WindowManagerCommandOutcome;
  /** Moves the window's tab to `index` within its pinned or unpinned range. */
  reorderTab(
    windowId: string,
    index: number,
    fence?: WindowManagerCommandFence
  ): WindowManagerCommandOutcome;
  /** Makes the window's tab this client's active tab and the desktop default. */
  activateTab(windowId: string): WindowManagerCommandOutcome;
  /** Pins or unpins the window's tab. */
  pinWindow(windowId: string, pinned: boolean): WindowManagerCommandOutcome;
  reopenWindow(): WindowManagerCommandOutcome;
  closeWindowScoped(windowId: string, scope: OsCloseScope): Promise<boolean>;
  setWallpaper(wallpaper: OsWallpaper): void;
  setReduceMotion(on: boolean): void;
  setDesktopBounds(bounds: OsDesktopBounds): void;
  /** Creates a desktop; callers that act on it next pass their own ID. */
  createDesktop(desktopId?: string): WindowManagerCommandOutcome;
  renameDesktop(desktopId: string, name: string): void;
  reorderDesktop(desktopId: string, order: number): void;
  switchDesktop(desktopId: string): void;
  switchDesktopDirection(direction: "previous" | "next"): void;
  deleteDesktop(desktopId: string, destinationId: string | null): void;
  /** Moves the window's whole tab, split included, to the end of another desktop's bar. */
  moveWindowToDesktop(windowId: string, destinationDesktopId: string): void;
  focusDirection(direction: FocusDirection): void;
  undoLayout(): void;
  redoLayout(): void;
  balanceFocusedLayout(): void;
  clearConflict(): void;
  refreshSnapshot(): Promise<boolean>;
}

export const OS_COMPACT_BREAKPOINT = 960;
/** Per-pane floor (shell-rail VC-03): seams and free edges stop at 220px on either axis. */
export const OS_WINDOW_MIN_WIDTH = 220;
export const OS_WINDOW_MIN_HEIGHT = 220;
