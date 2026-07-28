import { LAYOUT_CANVAS_REFERENCE } from "./window-manager-layout-reference";

/**
 * Ranges the web editor holds. The daemon is looser — it only demands finite,
 * positive thresholds (`internal/windowmanager/config.go`) — but a 600px edge
 * band would swallow the screen, so the controls stop where the value stops
 * being useful. These are the editor's limits, not the daemon's, and no copy on
 * the page claims otherwise.
 */
export const WINDOW_MANAGER_RANGES = {
  gap: { min: 0, max: 64 },
  edgeBand: { min: 4, max: 128 },
  cornerReach: { min: 16, max: 512 },
  exitSlack: { min: 0, max: 64 },
  historyLimit: { min: 1, max: 500 },
  repeatRatio: { min: 0.1, max: 0.9 },
  repeatStops: { min: 1, max: 8 },
} as const;

/**
 * The snap map is a scale model of the same reference screen the layout canvas
 * uses, so a band drawn here is the fraction of a real screen it will claim.
 */
export function snapMapScale(mapWidthPx: number): number {
  return mapWidthPx === 0 ? 1 : mapWidthPx / LAYOUT_CANVAS_REFERENCE.w;
}

export function snapMapPercent(valuePx: number, axis: "x" | "y"): string {
  const span = axis === "x" ? LAYOUT_CANVAS_REFERENCE.w : LAYOUT_CANVAS_REFERENCE.h;
  return `${(valuePx / span) * 100}%`;
}
