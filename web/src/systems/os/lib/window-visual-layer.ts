import type { OsWindowFrameModel } from "./group-projection";

export const WINDOW_VISUAL_LAYER = {
  tiled: 1,
  seam: 2,
} as const;

export function windowVisualLayer(frame: Pick<OsWindowFrameModel, "kind" | "layer">): number {
  return frame.kind === "floating"
    ? WINDOW_VISUAL_LAYER.seam + Math.max(frame.layer, 1)
    : WINDOW_VISUAL_LAYER.tiled;
}
