import { getOsApp, getOsAppMinimum, OS_APPS, OS_WINDOW_CONSERVATIVE_MINIMUM } from "./app-registry";
import { projectLayout } from "./layout-projection";
import { applySeamPreviewToDesktop, type SeamPreview } from "./seam-preview";
import type { SnapTargetConfig } from "./snap-targets";
import type { OsAppId, OsRect, OsWindow, OsWindowRoute } from "./os-types";
import type {
  LayoutProjection,
  NormalizedRect,
  PixelRect,
  WindowManagerClientView,
  WindowManagerConfig,
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
  return { pathname: getOsApp(app).paths[0] ?? "/", search: {} };
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
  return Object.hasOwn(OS_APPS, value) ? (value as OsAppId) : null;
}

export function buildWindowManagerWindows(input: {
  snapshot: WindowManagerSnapshot | null;
  client: WindowManagerClientView | null;
  workArea: PixelRect;
  projections: Readonly<Record<string, LayoutProjection>>;
  raiseOnFocus: boolean;
  routeIntents?: Readonly<Record<string, { route: OsWindowRoute }>>;
}): Readonly<Record<string, OsWindow>> {
  if (input.snapshot === null) return {};
  const windows: Record<string, OsWindow> = {};
  const focusLayerBase = Object.keys(input.snapshot.windows).length + 1;
  const projectionMaps = new Map(
    Object.entries(input.projections).map(([desktopId, projection]) => [
      desktopId,
      new Map(projection.windows.map(window => [window.windowId, window])),
    ])
  );
  const stableFloatingLayers = new Map<string, number>();
  for (const desktop of input.snapshot.desktops) {
    desktop.floating.forEach((windowId, index) => stableFloatingLayers.set(windowId, index));
  }
  for (const authoritative of Object.values(input.snapshot.windows)) {
    const resolvedApp = osAppId(authoritative.app);
    if (resolvedApp === null) continue;
    const projected = projectionMaps.get(authoritative.desktopId)?.get(authoritative.id);
    const focusIndex = input.raiseOnFocus
      ? (input.client?.focusOrder.indexOf(authoritative.id) ?? -1)
      : -1;
    const stableLayer = stableFloatingLayers.get(authoritative.id) ?? -1;
    const route = input.routeIntents?.[authoritative.id]?.route ?? authoritative.route;
    windows[authoritative.id] = {
      id: authoritative.id,
      app: resolvedApp,
      instanceKey: authoritative.instanceKey,
      route: {
        pathname: route.pathname,
        search: { ...route.search },
      },
      desktopId: authoritative.desktopId,
      placement: authoritative.placement,
      rect: projected?.rect ?? normalizedRectToPixels(authoritative.floatingRect, input.workArea),
      layer:
        focusIndex >= 0 ? Math.max(1, focusLayerBase - focusIndex) : Math.max(1, stableLayer + 1),
      minimized: authoritative.minimized,
      groupId: projected?.groupId ?? null,
      nodeId: projected?.nodeId ?? null,
      stackId: projected?.stackId ?? null,
      stackActive: projected?.active ?? true,
      parentAxis: projected?.parentAxis ?? null,
    };
  }
  return windows;
}
