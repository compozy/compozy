import type { OsWindow } from "./os-types";
import type { FocusDirection } from "./window-manager-types";

interface Point {
  x: number;
  y: number;
}

function center(window: OsWindow): Point {
  return {
    x: window.rect.x + window.rect.w / 2,
    y: window.rect.y + window.rect.h / 2,
  };
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

/** Visible peers eligible for an arrange command, ordered by current layer. */
export function arrangePeerWindows(
  windows: Readonly<Record<string, OsWindow>>,
  anchorId: string
): OsWindow[] {
  const anchor = windows[anchorId];
  if (!anchor) return [];
  return Object.values(windows)
    .filter(
      window => window.id !== anchorId && window.desktopId === anchor.desktopId && !window.minimized
    )
    .sort((left, right) => right.layer - left.layer || left.id.localeCompare(right.id));
}

/** Selects one deterministic visible focus target from projected geometry. */
export function directionalFocusTarget(input: {
  windows: readonly OsWindow[];
  focusedId: string | null;
  direction: FocusDirection;
  wrap: boolean;
}): string | null {
  const visible = input.windows.filter(window => !window.minimized && window.stackActive);
  if (visible.length === 0) return null;
  const source = visible.find(window => window.id === input.focusedId);
  if (!source) return visible[0]?.id ?? null;
  const sourceCenter = center(source);
  const directional: Array<{ id: string; distance: number }> = [];
  for (const window of visible) {
    if (window.id === source.id) continue;
    const distance = directionalDistance(sourceCenter, center(window), input.direction);
    if (distance !== null) directional.push({ id: window.id, distance });
  }
  directional.sort(
    (left, right) => left.distance - right.distance || left.id.localeCompare(right.id)
  );
  const nearest = directional[0];
  if (nearest) return nearest.id;
  if (!input.wrap) return null;
  return (
    visible
      .filter(window => window.id !== source.id)
      .sort((left, right) => {
        const distance =
          wrappedDistance(center(left), input.direction) -
          wrappedDistance(center(right), input.direction);
        return distance || left.id.localeCompare(right.id);
      })[0]?.id ?? null
  );
}
