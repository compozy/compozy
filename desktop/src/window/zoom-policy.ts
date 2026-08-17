export const MINIMUM_ZOOM_LEVEL = -3;
export const MAXIMUM_ZOOM_LEVEL = 3;

export function clampZoomLevel(level: number): number {
  if (!Number.isFinite(level)) return 0;
  return Math.min(MAXIMUM_ZOOM_LEVEL, Math.max(MINIMUM_ZOOM_LEVEL, Math.round(level)));
}

export function nextZoomLevel(current: number, direction: "in" | "out" | "reset"): number {
  if (direction === "reset") return 0;
  return clampZoomLevel(current + (direction === "in" ? 1 : -1));
}
