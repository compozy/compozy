import type { OsArrangePreset } from "./os-types";
import { leadWindowId, tabWindowIds } from "./tab-model";
import type { TabProjection } from "./tab-projection-types";
import type {
  FocusDirection,
  LayoutDesktop,
  LayoutNodeId,
  PixelRect,
  WindowId,
} from "./window-manager-types";

interface Point {
  x: number;
  y: number;
}

/** A window on screen and where its projection puts it. */
export interface FocusCandidate {
  id: WindowId;
  rect: PixelRect;
}

function center(rect: PixelRect): Point {
  return { x: rect.x + rect.w / 2, y: rect.y + rect.h / 2 };
}

/** Sub-pixel slack for edges that meet: a gutter-less split puts both panes on one line. */
const EDGE_TOLERANCE = 0.5;

/** How far `candidate` lies from `source` in `direction`; null when it is not past that edge. */
interface DirectionalFit {
  /** Shares part of the source's span on the other axis, rather than sitting diagonal to it. */
  overlaps: boolean;
  gap: number;
  centreDistance: number;
}

function spanOverlap(start: number, length: number, otherStart: number, otherLength: number) {
  return Math.min(start + length, otherStart + otherLength) - Math.max(start, otherStart);
}

/**
 * Measures `candidate` from `source` the way a tiling window manager moves focus: only a window past
 * the source's edge counts. Centres alone let a full-height window beside a stacked pair count as
 * "below" the top pane and steal focus from the pane under it.
 */
function directionalFit(
  source: PixelRect,
  candidate: PixelRect,
  direction: FocusDirection
): DirectionalFit | null {
  const horizontal = direction === "left" || direction === "right";
  const gap =
    direction === "left"
      ? source.x - (candidate.x + candidate.w)
      : direction === "right"
        ? candidate.x - (source.x + source.w)
        : direction === "up"
          ? source.y - (candidate.y + candidate.h)
          : candidate.y - (source.y + source.h);
  if (gap < -EDGE_TOLERANCE) return null;
  const overlap = horizontal
    ? spanOverlap(source.y, source.h, candidate.y, candidate.h)
    : spanOverlap(source.x, source.w, candidate.x, candidate.w);
  const from = center(source);
  const to = center(candidate);
  return {
    overlaps: overlap > EDGE_TOLERANCE,
    gap: Math.max(gap, 0),
    centreDistance: horizontal ? Math.abs(to.y - from.y) : Math.abs(to.x - from.x),
  };
}

/** Overlapping neighbours first, then the nearest edge, then the closest centre. */
function compareFit(left: DirectionalFit, right: DirectionalFit): number {
  if (left.overlaps !== right.overlaps) return left.overlaps ? -1 : 1;
  return left.gap - right.gap || left.centreDistance - right.centreDistance;
}

function wrappedDistance(candidate: Point, direction: FocusDirection): number {
  if (direction === "left") return -candidate.x;
  if (direction === "right") return candidate.x;
  if (direction === "up") return -candidate.y;
  return candidate.y;
}

/**
 * Windows one arrange gathers, anchor first: the rest of the anchor's tab, then the lead window of
 * every other tab on its desktop in bar order. Two-up takes one more window, grid caps at four.
 */
export function arrangeParticipants(
  desktop: LayoutDesktop,
  anchorId: WindowId,
  focusOrder: readonly WindowId[],
  preset: OsArrangePreset
): WindowId[] {
  const anchorTab = desktop.tabs.find(tab => new Set(tabWindowIds(tab)).has(anchorId));
  if (anchorTab === undefined) return [];
  const others = desktop.tabs.flatMap(tab => {
    if (tab.id === anchorTab.id) return [];
    const lead = leadWindowId(tab, focusOrder);
    return lead === null ? [] : [lead];
  });
  const peers = [...tabWindowIds(anchorTab).filter(id => id !== anchorId), ...others];
  switch (preset) {
    case "two-up":
      return [anchorId, ...peers.slice(0, 1)];
    case "grid":
      return [anchorId, ...peers.slice(0, 3)];
    case "main-stack":
    case "columns":
      return [anchorId, ...peers];
  }
}

/** Selects one deterministic on-screen focus target from projected geometry. */
export function directionalFocusTarget(input: {
  candidates: readonly FocusCandidate[];
  focusedId: string | null;
  direction: FocusDirection;
  wrap: boolean;
}): string | null {
  const visible = input.candidates;
  if (visible.length === 0) return null;
  const source = visible.find(candidate => candidate.id === input.focusedId);
  if (!source) return visible[0]?.id ?? null;
  const directional: Array<{ id: string; fit: DirectionalFit }> = [];
  for (const candidate of visible) {
    if (candidate.id === source.id) continue;
    const fit = directionalFit(source.rect, candidate.rect, input.direction);
    if (fit !== null) directional.push({ id: candidate.id, fit });
  }
  directional.sort(
    (left, right) => compareFit(left.fit, right.fit) || left.id.localeCompare(right.id)
  );
  const nearest = directional[0];
  if (nearest) return nearest.id;
  if (!input.wrap) return null;
  return (
    visible
      .filter(candidate => candidate.id !== source.id)
      .sort((left, right) => {
        const distance =
          wrappedDistance(center(left.rect), input.direction) -
          wrappedDistance(center(right.rect), input.direction);
        return distance || left.id.localeCompare(right.id);
      })[0]?.id ?? null
  );
}

/** The part of the desktop view that directional moves read: the active tab and its projection. */
export interface ActiveTabView {
  activeDesktopId: string | null;
  activeTabIds: Readonly<Record<string, string>>;
  projections: Readonly<Record<string, Pick<TabProjection, "windows">>>;
}

/** A window shown in the active tab, with the layout leaf that holds it. */
export interface ShownWindow extends FocusCandidate {
  /** The leaf's id; a swap exchanges the windows of two leaves and leaves their ids in place. */
  nodeId: LayoutNodeId;
}

/**
 * The windows shown on screen in the active tab, where the projection puts them. Members behind
 * a collapsed split's switcher are not candidates.
 */
export function activeTabCandidates(view: ActiveTabView): ShownWindow[] {
  if (view.activeDesktopId === null) return [];
  const tabId = view.activeTabIds[view.activeDesktopId];
  const projection = tabId === undefined ? undefined : view.projections[tabId];
  if (projection === undefined) return [];
  return projection.windows.flatMap(projected =>
    projected.shown
      ? [{ id: projected.windowId, rect: projected.rect, nodeId: projected.nodeId }]
      : []
  );
}

/**
 * The window a directional swap trades places with: the nearest of `candidates` on that side of
 * `fromId`, by the same geometry as directional focus but never wrapping. Null when `fromId` is not
 * among them or nothing lies on that side.
 */
export function directionalSwapTarget(
  candidates: readonly FocusCandidate[],
  fromId: WindowId | null,
  direction: FocusDirection
): WindowId | null {
  if (fromId === null || !candidates.some(candidate => candidate.id === fromId)) return null;
  return directionalFocusTarget({ candidates, focusedId: fromId, direction, wrap: false });
}
