import {
  getOsAppDescriptor,
  getOsAppMinimum,
  OS_WINDOW_CONSERVATIVE_MINIMUM,
  osAppId,
} from "./app-catalog";
import { applySeamPreviewToTab, type SeamPreview } from "./seam-preview";
import { OS_COMPACT_BREAKPOINT } from "./os-types";
import type {
  OsAppId,
  OsDesktopRuntimeStore,
  OsWallpaper,
  OsWindow,
  OsWindowRoute,
} from "./os-types";
import { projectTab } from "./tab-projection";
import { resolveActiveTabId } from "./tab-model";
import { effectiveWindowManagerConfig } from "./window-manager-config";
import type {
  DesktopId,
  LayoutNode,
  LayoutTab,
  PixelRect,
  ProjectedSeam,
  TabId,
  TabProjection,
  WindowManagerClientView,
  WindowManagerConfig,
  WindowManagerConnectionStatus,
  WindowManagerSnapshot,
  WindowMinimums,
} from "./window-manager-types";

export const DEFAULT_WINDOW_MANAGER_WORK_AREA: PixelRect = {
  x: 0,
  y: 0,
  w: 1440,
  h: 820,
};

export function defaultOsWindowRoute(app: OsAppId): OsWindowRoute {
  return { pathname: getOsAppDescriptor(app).paths[0] ?? "/", search: {} };
}

/**
 * Maps every authoritative window to the pixel floor its app declares. Typed against the shape it
 * reads rather than the whole snapshot, so a layout document gets identical minimums.
 */
export function buildWindowManagerMinimums(source: {
  windows: Readonly<Record<string, { id: string; app: string }>>;
}): WindowMinimums {
  return Object.fromEntries(
    Object.values(source.windows).map(window => {
      const app = osAppId(window.app);
      const minimum = app === null ? OS_WINDOW_CONSERVATIVE_MINIMUM : getOsAppMinimum(app);
      return [window.id, { width: minimum.width, height: minimum.height }];
    })
  );
}

/** This client's active tab per desktop: its own choice, else the default, else the first tab. */
export function buildActiveTabIds(
  snapshot: WindowManagerSnapshot | null,
  client: WindowManagerClientView | null
): Readonly<Record<DesktopId, TabId>> {
  const activeTabIds: Record<DesktopId, TabId> = {};
  for (const desktop of snapshot?.desktops ?? []) {
    const tabId = resolveActiveTabId(desktop, client?.activeTab);
    if (tabId !== null) activeTabIds[desktop.id] = tabId;
  }
  return activeTabIds;
}

interface ProjectionContext {
  workArea: PixelRect;
  config: WindowManagerConfig;
  minimums: WindowMinimums;
  focusOrder: readonly string[];
  compact: boolean;
}

function projectionInputFor(tab: LayoutTab, context: ProjectionContext) {
  return {
    tab,
    workArea: context.workArea,
    gaps: context.config.gaps,
    minimums: context.minimums,
    focusOrder: context.focusOrder,
    compact: context.compact,
  };
}

/**
 * The snapshot with a live seam drag applied to its split, so the rendered weights, the
 * projection and every view of the tab show the previewed shares until `layout.resize` commits.
 * The pixel delta is clamped against the tab's unpreviewed projection.
 */
function withSeamPreview(
  snapshot: WindowManagerSnapshot,
  context: ProjectionContext,
  preview: SeamPreview | null | undefined
): WindowManagerSnapshot {
  if (!preview) return snapshot;
  let changed = false;
  const desktops = snapshot.desktops.map(desktop => {
    const tabs = desktop.tabs.map(tab => {
      const seam = projectTab(projectionInputFor(tab, context)).seams.find(
        candidate =>
          candidate.splitId === preview.splitId && candidate.boundaryIndex === preview.boundaryIndex
      );
      if (seam === undefined) return tab;
      const previewed = applySeamPreviewToTab(tab, seam, preview.deltaPx);
      if (previewed !== tab) changed = true;
      return previewed;
    });
    return tabs.some((tab, index) => tab !== desktop.tabs[index]) ? { ...desktop, tabs } : desktop;
  });
  return changed ? { ...snapshot, desktops } : snapshot;
}

/**
 * One seam as the authoritative snapshot lays it out, before any live preview: the same seam a
 * seam preview is clamped against, so a gesture based on it commits exactly what it previews.
 * Null when the split or the boundary no longer exists.
 */
export function authoritativeSeam(
  view: Pick<OsDesktopRuntimeStore, "snapshot" | "windowManagerConfig" | "client" | "presentation">,
  workArea: PixelRect,
  target: { splitId: string; boundaryIndex: number }
): ProjectedSeam | null {
  const { snapshot, windowManagerConfig: config } = view;
  if (snapshot === null || config === null) return null;
  const context: ProjectionContext = {
    workArea,
    config,
    minimums: buildWindowManagerMinimums(snapshot),
    focusOrder: view.client?.focusOrder ?? [],
    compact: view.presentation === "compact",
  };
  for (const desktop of snapshot.desktops) {
    for (const tab of desktop.tabs) {
      const seam = projectTab(projectionInputFor(tab, context)).seams.find(
        candidate =>
          candidate.splitId === target.splitId && candidate.boundaryIndex === target.boundaryIndex
      );
      if (seam !== undefined) return seam;
    }
  }
  return null;
}

/** Every tab's split tree projected against the one measured content area. */
export function buildWindowManagerProjections(input: {
  snapshot: WindowManagerSnapshot | null;
  client: WindowManagerClientView | null;
  workArea: PixelRect;
  config: WindowManagerConfig | null;
  compact: boolean;
}): Readonly<Record<TabId, TabProjection>> {
  const { snapshot, config } = input;
  if (snapshot === null || config === null) return {};
  const context: ProjectionContext = {
    workArea: input.workArea,
    config,
    minimums: buildWindowManagerMinimums(snapshot),
    focusOrder: input.client?.focusOrder ?? [],
    compact: input.compact,
  };
  const projections: Record<TabId, TabProjection> = {};
  for (const desktop of snapshot.desktops) {
    for (const tab of desktop.tabs)
      projections[tab.id] = projectTab(projectionInputFor(tab, context));
  }
  return projections;
}

function leafNodeIds(node: LayoutNode, into: Map<string, string>): Map<string, string> {
  if (node.kind === "leaf") into.set(node.windowId, node.id);
  else for (const child of node.children) leafNodeIds(child, into);
  return into;
}

export function buildWindowManagerWindows(input: {
  snapshot: WindowManagerSnapshot | null;
  activeTabIds: Readonly<Record<DesktopId, TabId>>;
  projections: Readonly<Record<TabId, TabProjection>>;
  routeIntents?: Readonly<Record<string, { route: OsWindowRoute; instanceKey?: string | null }>>;
}): Readonly<Record<string, OsWindow>> {
  if (input.snapshot === null) return {};
  const tabs = new Map<TabId, LayoutTab>();
  for (const desktop of input.snapshot.desktops) {
    for (const tab of desktop.tabs) tabs.set(tab.id, tab);
  }
  const shown = new Map<string, boolean>();
  for (const projection of Object.values(input.projections)) {
    for (const projected of projection.windows) shown.set(projected.windowId, projected.shown);
  }
  const nodeIds = new Map<string, string>();
  for (const tab of tabs.values()) leafNodeIds(tab.root, nodeIds);
  const windows: Record<string, OsWindow> = {};
  for (const authoritative of Object.values(input.snapshot.windows)) {
    const app = osAppId(authoritative.app);
    if (app === null) continue;
    const tab = tabs.get(authoritative.tabId);
    const tabActive = input.activeTabIds[authoritative.desktopId] === authoritative.tabId;
    const routeIntent = input.routeIntents?.[authoritative.id];
    const route = routeIntent?.route ?? authoritative.route;
    windows[authoritative.id] = {
      id: authoritative.id,
      app,
      instanceKey: routeIntent?.instanceKey ?? authoritative.instanceKey,
      route: { pathname: route.pathname, search: { ...route.search } },
      navStack: authoritative.navStack.map(entry => ({
        pathname: entry.pathname,
        search: { ...entry.search },
      })),
      desktopId: authoritative.desktopId,
      tabId: authoritative.tabId,
      nodeId: nodeIds.get(authoritative.id) ?? null,
      pinned: tab?.pinned ?? false,
      inSplit: tab?.root.kind === "split",
      tabActive,
      shown: tabActive && (shown.get(authoritative.id) ?? true),
    };
  }
  return windows;
}

export interface OsDesktopRuntimeViewInput {
  viewportWidth?: number;
  snapshot: WindowManagerSnapshot | null;
  globalConfig: WindowManagerConfig | null;
  client: WindowManagerClientView | null;
  workArea: PixelRect;
  workAreaOrigin: { x: number; y: number };
  seamPreview: SeamPreview | null;
  routeIntents: Readonly<Record<string, { route: OsWindowRoute; instanceKey?: string | null }>>;
  connectionStatus: WindowManagerConnectionStatus;
  loadError: Error | null;
  wallpaper: OsWallpaper;
  reduceMotion: boolean;
}

/** Assembles the selector store the shell renders: active tabs → tab projections → windows. */
export function buildOsDesktopRuntimeView(input: OsDesktopRuntimeViewInput): OsDesktopRuntimeStore {
  const { snapshot, globalConfig, client, workArea } = input;
  const config =
    snapshot && globalConfig
      ? effectiveWindowManagerConfig(globalConfig, snapshot.overrides)
      : null;
  // Dock placement changes the work area; it must not change its own breakpoint.
  const compactViewport = (input.viewportWidth ?? workArea.w) < OS_COMPACT_BREAKPOINT;
  const compact = compactViewport && config?.smallViewportPolicy === "stack";
  const viewportRejected = compactViewport && config?.smallViewportPolicy === "reject";
  const live =
    snapshot && config
      ? withSeamPreview(
          snapshot,
          {
            workArea,
            config,
            minimums: buildWindowManagerMinimums(snapshot),
            focusOrder: client?.focusOrder ?? [],
            compact,
          },
          input.seamPreview
        )
      : null;
  const activeTabIds = buildActiveTabIds(live, client);
  const projections = buildWindowManagerProjections({
    snapshot: live,
    client,
    workArea,
    config,
    compact,
  });
  const windows = buildWindowManagerWindows({
    snapshot: live,
    activeTabIds,
    projections,
    routeIntents: input.routeIntents,
  });
  const ready = snapshot !== null && config !== null;
  let hydration: OsDesktopRuntimeStore["hydration"] = ready ? "live" : "pending";
  if (input.loadError) hydration = "degraded";
  return {
    snapshot,
    windowManagerConfig: config,
    client,
    clientAttachmentToken: null,
    // The rendered desktops carry a live seam preview; `snapshot` stays authoritative.
    desktops: (live ?? snapshot)?.desktops ?? [],
    activeTabIds,
    projections,
    windows,
    activeDesktopId: client?.activeDesktopId ?? snapshot?.desktops[0]?.id ?? null,
    focusedId: client?.focusedWindowId ?? null,
    wallpaper: input.wallpaper,
    reduceMotion: input.reduceMotion,
    presentation: compact ? "compact" : "split",
    viewportState: viewportRejected ? "rejected" : "ready",
    hydration,
    loadError: input.loadError,
    connectionStatus: input.connectionStatus,
    desktopBounds: {
      width: workArea.w,
      height: workArea.h,
      origin: input.workAreaOrigin,
    },
  };
}
