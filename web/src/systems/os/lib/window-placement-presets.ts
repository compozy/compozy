import type { OsArrangePreset, WindowManagerController } from "./os-types";
import type { SnapCorner, SnapSide } from "./snap-targets";

export type WindowPlacementId = SnapSide | SnapCorner;

export interface WindowPlacementCommand {
  id: `window.tile.${WindowPlacementId}`;
  placement: WindowPlacementId;
  label: string;
}

export interface WindowArrangeCommand {
  id: `layout.arrange.${OsArrangePreset}`;
  preset: OsArrangePreset;
  label: string;
}

/**
 * Geometry presets for direct manipulation — the zoom menu tiles the window it
 * is attached to, not whichever window happens to be focused.
 *
 * This is not a command catalog: the palette's and menubar's tiling rows come
 * from the registry projection, and their ids match these only because both
 * describe the same geometry. Nothing here is invokable by id.
 */
export const WINDOW_PLACEMENT_COMMANDS: readonly WindowPlacementCommand[] = [
  { id: "window.tile.left", placement: "left", label: "Tile left half" },
  { id: "window.tile.right", placement: "right", label: "Tile right half" },
  { id: "window.tile.top", placement: "top", label: "Tile top half" },
  { id: "window.tile.bottom", placement: "bottom", label: "Tile bottom half" },
  { id: "window.tile.top-left", placement: "top-left", label: "Tile top left quarter" },
  { id: "window.tile.top-right", placement: "top-right", label: "Tile top right quarter" },
  { id: "window.tile.bottom-left", placement: "bottom-left", label: "Tile bottom left quarter" },
  { id: "window.tile.bottom-right", placement: "bottom-right", label: "Tile bottom right quarter" },
];

export const WINDOW_ARRANGE_COMMANDS: readonly WindowArrangeCommand[] = [
  { id: "layout.arrange.two-up", preset: "two-up", label: "Arrange left & right" },
  { id: "layout.arrange.grid", preset: "grid", label: "Arrange in grid" },
];

export function dispatchWindowPlacement(
  manager: WindowManagerController,
  windowId: string,
  command: WindowPlacementCommand
): void {
  manager.tileWindow(windowId, command.placement);
}
