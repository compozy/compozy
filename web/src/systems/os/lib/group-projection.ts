import { frameResizableEdges, type TiledResizableEdges } from "./tiled-resize";
import { windowManagerLayoutArea } from "./window-manager-layout-area";
import type {
  DesktopId,
  LayoutDesktop,
  LayoutNodeId,
  LayoutProjection,
  NormalizedRect,
  PixelRect,
  ProjectionGaps,
  WindowId,
  WindowManagerClientView,
  WindowManagerSnapshot,
} from "./window-manager-types";

const ALL_EDGES_RESIZABLE: TiledResizableEdges = {
  left: true,
  right: true,
  top: true,
  bottom: true,
};
const NO_EDGES_RESIZABLE: TiledResizableEdges = {
  left: false,
  right: false,
  top: false,
  bottom: false,
};

/**
 * One rendered frame per window group (ADR-001): a tiled pane, a floating
 * window, or a floating tab stack. The frame hosts the deck at ≥2 members and
 * every member's surface — the projection is the single place that decides
 * membership, deck order, and the display-active member.
 */
export interface OsWindowFrameModel {
  /** Stack node id, floating-stack id, or the solo member's window id. */
  id: string;
  desktopId: DesktopId;
  kind: "tiled" | "floating";
  rect: PixelRect;
  /** Deck order: the daemon collates the pinned prefix first. */
  members: readonly WindowId[];
  activeWindowId: WindowId;
  /** Real stack identity for stack commands; null for solo frames. */
  stackId: LayoutNodeId | null;
  minimized: boolean;
  /** The unit fills the desktop work area; drag and resize are off until it unzooms. */
  zoomed: boolean;
  /** Pane too small for its split — projection degraded it to a stack. */
  adapted: boolean;
  /** Stacking layer among floating frames; tiled frames never overlap. */
  layer: number;
  /** Normalized gap-free zone a tiled unit owns; null for floating frames. */
  zone: NormalizedRect | null;
  /** Free edges resize this unit alone; shared edges resize through seams. */
  resizableEdges: TiledResizableEdges;
}

function normalizedRectToPixels(rect: NormalizedRect, area: PixelRect): PixelRect {
  return {
    x: Math.round(area.x + rect.x * area.w),
    y: Math.round(area.y + rect.y * area.h),
    w: Math.max(1, Math.round(rect.w * area.w)),
    h: Math.max(1, Math.round(rect.h * area.h)),
  };
}

/** Most-recently-focused member ranks the whole frame (newest-first order). */
function frameLayer(
  members: readonly WindowId[],
  focusOrder: readonly WindowId[],
  layerBase: number,
  stableIndex: number
): number {
  let bestRank = Number.POSITIVE_INFINITY;
  for (const member of members) {
    const index = focusOrder.indexOf(member);
    if (index >= 0 && index < bestRank) bestRank = index;
  }
  if (bestRank === Number.POSITIVE_INFINITY) return Math.max(1, stableIndex + 1);
  return Math.max(1, layerBase - bestRank);
}

function displayActive(
  members: readonly WindowId[],
  durableActiveId: WindowId | null,
  clientActive: WindowId | undefined
): WindowId {
  if (clientActive !== undefined && members.includes(clientActive)) return clientActive;
  if (durableActiveId !== null && members.includes(durableActiveId)) return durableActiveId;
  return members[0] ?? "";
}

function tiledFrames(desktop: LayoutDesktop, projection: LayoutProjection): OsWindowFrameModel[] {
  const frames: OsWindowFrameModel[] = [];
  const units: Array<{ id: string; zone: NormalizedRect }> = [
    ...projection.stacks.map(stack => ({ id: stack.nodeId, zone: stack.zone })),
    ...projection.windows.flatMap(window =>
      window.stackId === null ? [{ id: window.windowId, zone: window.zone }] : []
    ),
  ];
  const resizableEdgesFor = (unitId: string, zone: NormalizedRect, adapted: boolean) =>
    adapted
      ? NO_EDGES_RESIZABLE
      : frameResizableEdges(
          zone,
          units.flatMap(unit => (unit.id === unitId ? [] : [unit.zone]))
        );
  for (const stack of projection.stacks) {
    frames.push({
      id: stack.nodeId,
      desktopId: desktop.id,
      kind: "tiled",
      rect: stack.rect,
      members: stack.windowIds,
      activeWindowId: stack.activeWindowId,
      stackId: stack.kind === "explicit" ? stack.nodeId : null,
      minimized: false,
      zoomed: false,
      adapted: stack.kind === "adaptive",
      layer: 1,
      zone: stack.zone,
      resizableEdges: resizableEdgesFor(stack.nodeId, stack.zone, stack.kind === "adaptive"),
    });
  }
  for (const window of projection.windows) {
    if (window.stackId !== null) continue;
    frames.push({
      id: window.windowId,
      desktopId: desktop.id,
      kind: "tiled",
      rect: window.rect,
      members: [window.windowId],
      activeWindowId: window.windowId,
      stackId: null,
      minimized: false,
      zoomed: false,
      adapted: false,
      layer: 1,
      zone: window.zone,
      resizableEdges: resizableEdgesFor(window.windowId, window.zone, false),
    });
  }
  return frames;
}

function floatingFrames(input: {
  desktop: LayoutDesktop;
  snapshot: WindowManagerSnapshot;
  client: WindowManagerClientView | null;
  workArea: PixelRect;
  layerBase: number;
  raiseOnFocus: boolean;
}): OsWindowFrameModel[] {
  const { desktop, snapshot, client, workArea, layerBase } = input;
  const focusOrder = input.raiseOnFocus ? (client?.focusOrder ?? []) : [];
  const frames: OsWindowFrameModel[] = [];
  desktop.floating.forEach((windowId, index) => {
    const window = snapshot.windows[windowId];
    if (window === undefined) return;
    frames.push({
      id: windowId,
      desktopId: desktop.id,
      kind: "floating",
      rect: normalizedRectToPixels(window.floatingRect, workArea),
      members: [windowId],
      activeWindowId: windowId,
      stackId: null,
      minimized: window.minimized,
      zoomed: false,
      adapted: false,
      layer: frameLayer([windowId], focusOrder, layerBase, index),
      zone: null,
      resizableEdges: ALL_EDGES_RESIZABLE,
    });
  });
  desktop.floatingStacks.forEach((stack, index) => {
    if (stack.windowIds.length === 0) return;
    frames.push({
      id: stack.id,
      desktopId: desktop.id,
      kind: "floating",
      rect: normalizedRectToPixels(stack.rect, workArea),
      members: stack.windowIds,
      activeWindowId: displayActive(stack.windowIds, stack.activeId, client?.stackActive[stack.id]),
      stackId: stack.id,
      minimized: stack.minimized,
      zoomed: false,
      adapted: false,
      layer: frameLayer(stack.windowIds, focusOrder, layerBase, desktop.floating.length + index),
      zone: null,
      resizableEdges: ALL_EDGES_RESIZABLE,
    });
  });
  return frames;
}

/**
 * Projects every desktop's frames in one pass over the snapshot and the
 * already-computed layout projections — no per-member recomputation, which is
 * what keeps hundreds of tabs honest at the projection level.
 */
export function buildDesktopFrames(input: {
  snapshot: WindowManagerSnapshot | null;
  client: WindowManagerClientView | null;
  projections: Readonly<Record<DesktopId, LayoutProjection>>;
  workArea: PixelRect;
  gaps: ProjectionGaps;
  raiseOnFocus: boolean;
}): Readonly<Record<DesktopId, readonly OsWindowFrameModel[]>> {
  if (input.snapshot === null) return {};
  const layerBase = Object.keys(input.snapshot.windows).length + 1;
  const zoomRect = windowManagerLayoutArea(input.workArea, input.gaps);
  const frames: Record<DesktopId, readonly OsWindowFrameModel[]> = {};
  for (const desktop of input.snapshot.desktops) {
    const projection = input.projections[desktop.id];
    const desktopFrames = [
      ...(projection ? tiledFrames(desktop, projection) : []),
      ...floatingFrames({
        desktop,
        snapshot: input.snapshot,
        client: input.client,
        workArea: input.workArea,
        layerBase,
        raiseOnFocus: input.raiseOnFocus,
      }),
    ];
    frames[desktop.id] = zoomFrames(desktopFrames, input.snapshot, zoomRect);
  }
  return frames;
}

/**
 * The unit holding a zoomed window takes the whole layout area and gives up
 * its own drag and resize affordances; every other frame keeps its place so
 * unzooming reveals the tree exactly as it was.
 */
function zoomFrames(
  frames: readonly OsWindowFrameModel[],
  snapshot: WindowManagerSnapshot,
  zoomRect: PixelRect
): readonly OsWindowFrameModel[] {
  return frames.map(frame => {
    const zoomed =
      !frame.minimized &&
      frame.members.some(member => {
        const window = snapshot.windows[member];
        return window !== undefined && window.zoomed && !window.minimized;
      });
    if (!zoomed) return frame;
    return { ...frame, rect: { ...zoomRect }, zoomed: true, resizableEdges: NO_EDGES_RESIZABLE };
  });
}

/** The zoomed frame of a desktop, if one member is zoomed. */
export function zoomedFrame(
  frames: readonly OsWindowFrameModel[] | undefined
): OsWindowFrameModel | null {
  return frames?.find(frame => frame.zoomed) ?? null;
}

/** The frame a window currently belongs to, if any (deck lookups, gestures). */
export function frameForWindow(
  frames: Readonly<Record<DesktopId, readonly OsWindowFrameModel[]>>,
  windowId: WindowId
): OsWindowFrameModel | null {
  for (const desktopFrames of Object.values(frames)) {
    for (const frame of desktopFrames) {
      if (frame.members.includes(windowId)) return frame;
    }
  }
  return null;
}
