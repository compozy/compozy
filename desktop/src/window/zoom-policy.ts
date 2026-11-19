export const MINIMUM_ZOOM_LEVEL = -3;
export const MAXIMUM_ZOOM_LEVEL = 3;

export function clampZoomLevel(level: number): number {
  if (!Number.isFinite(level)) return 0;
  return Math.min(MAXIMUM_ZOOM_LEVEL, Math.max(MINIMUM_ZOOM_LEVEL, Math.round(level)));
}

export function nextZoomLevel(current: number, direction: ZoomDirection): number {
  if (direction === "reset") return 0;
  return clampZoomLevel(current + (direction === "in" ? 1 : -1));
}

export type ZoomDirection = "in" | "out" | "reset";

export interface ZoomKeyInput {
  readonly type: string;
  readonly key: string;
  readonly control: boolean;
  readonly meta: boolean;
}

// Zoom uses the platform's primary modifier only: Cmd on macOS, Ctrl elsewhere.
// On macOS Ctrl+0 / Ctrl+= / Ctrl+- belong to the focused surface (a terminal shell).
export function zoomDirectionForInput(
  input: ZoomKeyInput,
  platform: NodeJS.Platform
): ZoomDirection | null {
  if (input.type !== "keyDown") return null;
  const apple = platform === "darwin";
  const primary = apple ? input.meta && !input.control : input.control && !input.meta;
  if (!primary) return null;
  if (input.key === "0") return "reset";
  if (input.key === "+" || input.key === "=") return "in";
  if (input.key === "-") return "out";
  return null;
}
