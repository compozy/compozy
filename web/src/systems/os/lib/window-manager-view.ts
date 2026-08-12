import {
  getOsAppDescriptor,
  getOsAppMinimum,
  OS_APP_DESCRIPTORS,
  OS_WINDOW_CONSERVATIVE_MINIMUM,
} from "./app-catalog";
import { applyFrameSeamPreviewToDesktop } from "./frame-seams";
import { buildDesktopFrames, type OsWindowFrameModel } from "./group-projection";
import { projectLayout } from "./layout-projection";
import { applySeamPreviewToDesktop, type SeamPreview } from "./seam-preview";
import type { SnapTargetConfig } from "./snap-targets";
import { OS_COMPACT_BREAKPOINT } from "./os-types";
import type {
  OsAppId,
  OsDesktopRuntimeStore,
  OsRect,
  OsWallpaper,
  OsWindow,
  OsWindowRoute,
} from "./os-types";
import { effectiveWindowManagerConfig } from "./window-manager-config";
import type {
  DesktopId,
  LayoutProjection,
  NormalizedRect,
  PixelRect,
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
export const DEFAULT_WINDOW_MANAGER_FLOATING_RECT: NormalizedRect = {
  x: 0.12,
  y: 0.08,
  w: 0.68,
  h: 0.78,
};

/**
 * Pointer resolution requires a complete live configuration. Workspace
 * overrides are intentionally not merged with browser-owned defaults here.
 */
export function snapTargetConfigFromConfig(
  config: WindowManagerConfig | null
): SnapTargetConfig | null {
  if (config === null) return null;
  return {
    edgeBand: config.snap.edgeBand,
    cornerReach: config.snap.cornerReach,
    exitSlack: config.snap.exitSlack,
    innerGap: config.gaps.inner,
    outerGaps: {
      top: config.gaps.top,
      right: config.gaps.right,
      bottom: config.gaps.bottom,
      left: config.gaps.left,
    },
    repeatRatios: config.snap.repeatRatios,
    topCenter: config.bindings.topCenter,
    bottomCenter: config.bindings.bottomCenter,
  };
}

export function defaultOsWindowRoute(app: OsAppId): OsWindowRoute {
  return { pathname: getOsAppDescriptor(app).paths[0] ?? "/", search: {} };
}

export function pixelRectToNormalized(rect: OsRect, area: PixelRect): NormalizedRect {
  if (area.w <= 0 || area.h <= 0) return DEFAULT_WINDOW_MANAGER_FLOATING_RECT;
  return {
    x: Math.max(0, Math.min(1, (rect.x - area.x) / area.w)),
    y: Math.max(0, Math.min(1, (rect.y - area.y) / area.h)),
    w: Math.max(0.01, Math.min(1, rect.w / area.w)),
    h: Math.max(0.01, Math.min(1, rect.h / area.h)),
  };
}

export function normalizedRectToWire(rect: NormalizedRect): Record<string, number> {
  return { x: rect.x, y: rect.y, width: rect.w, height: rect.h };
}

/** Projects one desktop, re-projecting against the unadjusted seam when a live preview is active. */
function projectDesktopWithSeamPreview(
  input: Parameters<typeof projectLayout>[0],
  seamPreview: SeamPreview | null
): LayoutProjection {
  const projection = projectLayout(input);
  if (seamPreview === null) return projection;
  if (seamPreview.kind === "frame") {
    const frameSeam = projection.frameSeams.find(candidate => candidate.id === seamPreview.seamId);
    if (frameSeam === undefined) return projection;
    return projectLayout({
      ...input,
      desktop: applyFrameSeamPreviewToDesktop(input.desktop, frameSeam, seamPreview.deltaPx),
    });
  }
  const seam = projection.seams.find(
    candidate =>
      candidate.splitId === seamPreview.splitId &&
      candidate.boundaryIndex === seamPreview.boundaryIndex
  );
  if (seam === undefined) return projection;
  return projectLayout({
    ...input,
    desktop: applySeamPreviewToDesktop(input.desktop, seam, seamPreview.deltaPx),
  });
}

export function buildWindowManagerProjections(
  snapshot: WindowManagerSnapshot | null,
  client: WindowManagerClientView | null,
  workArea: PixelRect,
  config: WindowManagerConfig | null,
  seamPreview: SeamPreview | null = null
): Readonly<Record<string, LayoutProjection>> {
  if (snapshot === null || config === null) return {};
  const projections: Record<string, LayoutProjection> = {};
  const minimums = buildWindowManagerMinimums(snapshot);
  for (const desktop of snapshot.desktops) {
    projections[desktop.id] = projectDesktopWithSeamPreview(
      {
        revision: snapshot.revision,
        desktop,
        workArea,
        gaps: config.gaps,
        minimums,
        focusedWindowId: client?.activeDesktopId === desktop.id ? client.focusedWindowId : null,
        stackActive: client?.stackActive ?? {},
      },
      seamPreview
    );
  }
  return projections;
}

/**
 * Maps every authoritative window to the same floor used by its interactive
 * frame. Typed against the shape it reads rather than the whole snapshot, so the
 * settings editor can pass a layout document and get identical minimums — the
 * projector's `minimum-unmet` and `adaptive-stack` diagnostics only mean
 * something if both surfaces measure against the same floor.
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

function normalizedRectToPixels(rect: NormalizedRect, area: PixelRect): OsRect {
  return {
    x: Math.round(area.x + rect.x * area.w),
    y: Math.round(area.y + rect.y * area.h),
    w: Math.max(1, Math.round(rect.w * area.w)),
    h: Math.max(1, Math.round(rect.h * area.h)),
  };
}

function osAppId(value: string): OsAppId | null {
  return Object.hasOwn(OS_APP_DESCRIPTORS, value) ? (value as OsAppId) : null;
}

export interface OsDesktopRuntimeViewInput {
  snapshot: WindowManagerSnapshot | null;
  globalConfig: WindowManagerConfig | null;
  client: WindowManagerClientView | null;
  workArea: PixelRect;
  workAreaOrigin: { x: number; y: number };
  seamPreview: SeamPreview | null;
  routeIntents: Readonly<Record<string, { route: OsWindowRoute }>>;
  connectionStatus: WindowManagerConnectionStatus;
  loadError: Error | null;
  railCollapsedAgentIds: readonly string[];
  wallpaper: OsWallpaper;
  reduceMotion: boolean;
  dockMagnify: boolean;
}

/** Assembles the selector store the shell renders: projections → frames → windows. */
export function buildOsDesktopRuntimeView(input: OsDesktopRuntimeViewInput): OsDesktopRuntimeStore {
  const { snapshot, globalConfig, client, workArea } = input;
  const config =
    snapshot && globalConfig
      ? effectiveWindowManagerConfig(globalConfig, snapshot.overrides)
      : null;
  const projections = buildWindowManagerProjections(
    snapshot,
    client,
    workArea,
    config,
    input.seamPreview
  );
  const frames = buildDesktopFrames({
    snapshot: config ? snapshot : null,
    client,
    projections,
    workArea,
    raiseOnFocus: config?.raiseOnFocus ?? false,
  });
  const windows = buildWindowManagerWindows({
    snapshot: config ? snapshot : null,
    client,
    workArea,
    projections,
    frames,
    routeIntents: input.routeIntents,
  });
  const hydration =
    snapshot !== null && config !== null
      ? input.loadError
        ? "degraded"
        : "live"
      : input.loadError
        ? "degraded"
        : "pending";
  const viewportRejected =
    workArea.w < OS_COMPACT_BREAKPOINT && config?.smallViewportPolicy === "reject";
  return {
    snapshot,
    windowManagerConfig: config,
    client,
    desktops: snapshot?.desktops ?? [],
    projections,
    frames,
    windows,
    activeDesktopId: client?.activeDesktopId ?? snapshot?.desktops[0]?.id ?? null,
    focusedId: client?.focusedWindowId ?? null,
    railCollapsedAgentIds: input.railCollapsedAgentIds,
    wallpaper: input.wallpaper,
    reduceMotion: input.reduceMotion,
    dockMagnify: input.dockMagnify,
    presentation:
      workArea.w < OS_COMPACT_BREAKPOINT && config?.smallViewportPolicy === "stack"
        ? "compact"
        : "floating",
    viewportState: viewportRejected ? "rejected" : "ready",
    hydration,
    connectionStatus: input.connectionStatus,
    desktopBounds: {
      width: workArea.w,
      height: workArea.h,
      origin: input.workAreaOrigin,
    },
  };
}

export function buildWindowManagerWindows(input: {
  snapshot: WindowManagerSnapshot | null;
  client: WindowManagerClientView | null;
  workArea: PixelRect;
  projections: Readonly<Record<string, LayoutProjection>>;
  frames: Readonly<Record<DesktopId, readonly OsWindowFrameModel[]>>;
  routeIntents?: Readonly<Record<string, { route: OsWindowRoute }>>;
}): Readonly<Record<string, OsWindow>> {
  if (input.snapshot === null) return {};
  const windows: Record<string, OsWindow> = {};
  const projectionMaps = new Map(
    Object.entries(input.projections).map(([desktopId, projection]) => [
      desktopId,
      new Map(projection.windows.map(window => [window.windowId, window])),
    ])
  );
  const frameByMember = new Map<string, OsWindowFrameModel>();
  for (const desktopFrames of Object.values(input.frames)) {
    for (const frame of desktopFrames) {
      for (const member of frame.members) frameByMember.set(member, frame);
    }
  }
  for (const authoritative of Object.values(input.snapshot.windows)) {
    const resolvedApp = osAppId(authoritative.app);
    if (resolvedApp === null) continue;
    const projected = projectionMaps.get(authoritative.desktopId)?.get(authoritative.id);
    const frame = frameByMember.get(authoritative.id);
    const route = input.routeIntents?.[authoritative.id]?.route ?? authoritative.route;
    windows[authoritative.id] = {
      id: authoritative.id,
      app: resolvedApp,
      instanceKey: authoritative.instanceKey,
      route: {
        pathname: route.pathname,
        search: { ...route.search },
      },
      navStack: authoritative.navStack.map(entry => ({
        pathname: entry.pathname,
        search: { ...entry.search },
      })),
      pinned: authoritative.pinned,
      desktopId: authoritative.desktopId,
      placement: authoritative.placement,
      rect: frame?.rect ?? normalizedRectToPixels(authoritative.floatingRect, input.workArea),
      layer: frame?.layer ?? 1,
      minimized: authoritative.minimized || (frame?.minimized ?? false),
      groupId: projected?.groupId ?? null,
      nodeId: projected?.nodeId ?? null,
      stackId: frame?.stackId ?? null,
      stackActive: frame === undefined || frame.activeWindowId === authoritative.id,
      parentAxis: projected?.parentAxis ?? null,
    };
  }
  return windows;
}
