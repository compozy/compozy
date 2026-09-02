import type { OsWindowFrameModel } from "./group-projection";

export const WINDOW_VISUAL_LAYER = {
  tiled: 1,
  seam: 2,
  /** A zoomed tiled unit covers its tree and seams; floating frames stay above it. */
  zoomed: 3,
} as const;

export function windowVisualLayer(
  frame: Pick<OsWindowFrameModel, "kind" | "layer" | "zoomed">
): number {
  if (frame.kind === "floating") return WINDOW_VISUAL_LAYER.zoomed + Math.max(frame.layer, 1);
  return frame.zoomed ? WINDOW_VISUAL_LAYER.zoomed : WINDOW_VISUAL_LAYER.tiled;
}
