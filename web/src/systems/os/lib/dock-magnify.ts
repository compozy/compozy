/**
 * OpenDesign dock magnification math (os-v2.js).
 * Linear horizontal falloff → translateY + scale from bottom-center.
 */

export const DOCK_MAG_RADIUS = 96;
export const DOCK_MAG_SCALE = 0.34;
export const DOCK_MAG_LIFT_PX = 7;

/** Influence factor in [0, 1] from horizontal distance to an icon center. */
export function dockMagnifyFactor(distancePx: number, radius: number = DOCK_MAG_RADIUS): number {
  if (radius <= 0) return 0;
  return Math.max(0, 1 - distancePx / radius);
}

/** CSS transform for a magnification factor (prototype formula). */
export function dockMagnifyTransform(
  factor: number,
  scaleAmp: number = DOCK_MAG_SCALE,
  liftPx: number = DOCK_MAG_LIFT_PX
): string {
  if (factor <= 0) return "";
  return `translateY(${-liftPx * factor}px) scale(${1 + scaleAmp * factor})`;
}
