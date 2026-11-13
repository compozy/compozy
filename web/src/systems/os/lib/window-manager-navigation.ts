import type { OsArrangePreset } from "./os-types";
import { leadWindowId, tabWindowIds } from "./tab-model";
import type { TabProjection } from "./tab-projection-types";
import type { FocusDirection, LayoutDesktop, PixelRect, WindowId } from "./window-manager-types";

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

function directionalDistance(
  source: Point,
  candidate: Point,
  direction: FocusDirection
): number | null {
  const horizontal = candidate.x - source.x;
  const vertical = candidate.y - source.y;
  const primary =
    direction === "left"
      ? -horizontal
      : direction === "right"
        ? horizontal
        : direction === "up"
          ? -vertical
          : vertical;
  if (primary <= 0) return null;
  const secondary =
    direction === "left" || direction === "right" ? Math.abs(vertical) : Math.abs(horizontal);
  return primary * 1_000 + secondary;
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
  const sourceCenter = center(source.rect);
  const directional: Array<{ id: string; distance: number }> = [];
  for (const candidate of visible) {
    if (candidate.id === source.id) continue;
    const distance = directionalDistance(sourceCenter, center(candidate.rect), input.direction);
    if (distance !== null) directional.push({ id: candidate.id, distance });
  }
  directional.sort(
    (left, right) => left.distance - right.distance || left.id.localeCompare(right.id)
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

/**
 * The windows shown on screen in the active tab, where the projection puts them. Members behind
 * a collapsed split's switcher are not candidates.
 */
export function activeTabCandidates(view: ActiveTabView): FocusCandidate[] {
  if (view.activeDesktopId === null) return [];
  const tabId = view.activeTabIds[view.activeDesktopId];
  const projection = tabId === undefined ? undefined : view.projections[tabId];
  if (projection === undefined) return [];
  return projection.windows.flatMap(projected =>
    projected.shown ? [{ id: projected.windowId, rect: projected.rect }] : []
  );
}

/**
 * The window a directional swap trades places with: the nearest on-screen window of the active
 * tab on that side of `fromId`, by the same geometry as directional focus but never wrapping.
 * Null when `fromId` is not on screen there or nothing lies on that side.
 */
export function directionalSwapTarget(
  view: ActiveTabView,
  fromId: WindowId | null,
  direction: FocusDirection
): WindowId | null {
  const candidates = activeTabCandidates(view);
  if (fromId === null || !candidates.some(candidate => candidate.id === fromId)) return null;
  return directionalFocusTarget({ candidates, focusedId: fromId, direction, wrap: false });
}
