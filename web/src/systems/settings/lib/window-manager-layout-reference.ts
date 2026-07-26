import type { PixelRect } from "@/systems/os";

/**
 * Constants only, with no value import — the settings barrel and the OS barrel
 * import each other, so anything read at module scope has to be reachable
 * without waiting for that cycle to unwind.
 */

/**
 * The canvas is a 1:1 scale model of one screen, not a preview of the current
 * browser window. Projecting into a fixed reference viewport is what lets the
 * gaps read in real pixels and lets the projector's fit diagnostics carry an
 * honest caption — "at 1440 × 900" — instead of implying they describe whatever
 * viewport happens to be showing Settings.
 */
export const LAYOUT_CANVAS_REFERENCE: PixelRect = { x: 0, y: 0, w: 1440, h: 900 };

/**
 * The narrowest a group may be dragged. The daemon only demands a positive span,
 * but a group under a twelfth of the screen cannot hold a window at any real
 * viewport, so the handle stops there instead of letting someone build a layout
 * that only fails later.
 */
export const LAYOUT_GROUP_MIN_SPAN = 0.08;
