import type { OsArrangePreset, WindowManagerController } from "./os-types";
import type { SnapCorner, SnapSide } from "./snap-targets";

export type WindowPlacementId = SnapSide | SnapCorner;

export interface WindowPlacementPreset {
  placement: WindowPlacementId;
}

export interface WindowArrangePreset {
  preset: OsArrangePreset;
}

/**
 * Geometry presets for direct manipulation — the zoom menu tiles the window it
 * is attached to, not whichever window happens to be focused.
 *
 * This is not a command catalog: the palette's and menubar's tiling rows come
 * from the registry projection. IDs here are only derived so the menu can look
 * up the live title. Nothing here is invokable by id.
 */
export const WINDOW_PLACEMENT_PRESETS: readonly WindowPlacementPreset[] = [
  { placement: "left" },
  { placement: "right" },
  { placement: "top" },
  { placement: "bottom" },
  { placement: "top-left" },
  { placement: "top-right" },
  { placement: "bottom-left" },
  { placement: "bottom-right" },
];

export const WINDOW_ARRANGE_PRESETS: readonly WindowArrangePreset[] = [
  { preset: "two-up" },
  { preset: "grid" },
];

export function windowPlacementCommandId(
  placement: WindowPlacementId
): `window.tile.${WindowPlacementId}` {
  return `window.tile.${placement}`;
}

export function windowArrangeCommandId(
  preset: OsArrangePreset
): `layout.arrange.${OsArrangePreset}` {
  return `layout.arrange.${preset}`;
}

export function dispatchWindowPlacement(
  manager: WindowManagerController,
  windowId: string,
  preset: WindowPlacementPreset
): void {
  manager.tileWindow(windowId, preset.placement);
}
