import type { GestureCancelReason } from "./layout-gesture-session";
import type { PixelPoint } from "./window-manager-types";

type CancelGesture = (reason: GestureCancelReason, point?: PixelPoint) => unknown;

function pointerPoint(event: PointerEvent): PixelPoint {
  return { x: event.clientX, y: event.clientY };
}

function cancelledTouchPoint(event: TouchEvent): PixelPoint | undefined {
  const touch = event.changedTouches[0];
  return touch ? { x: touch.clientX, y: touch.clientY } : undefined;
}

/** Binds every browser-level terminal signal that invalidates an active pointer gesture. */
export function bindLayoutGestureCancellation(
  target: Window,
  cancelGesture: CancelGesture
): () => void {
  const handleEscape = (event: KeyboardEvent) => {
    if (event.key !== "Escape") return;
    event.preventDefault();
    cancelGesture("escape");
  };
  const handlePointerCancel = (event: PointerEvent) => {
    cancelGesture("pointer-cancel", pointerPoint(event));
  };
  const handleTouchCancel = (event: TouchEvent) => {
    cancelGesture("pointer-cancel", cancelledTouchPoint(event));
  };
  const handleLostPointerCapture = () => {
    cancelGesture("lost-capture");
  };

  target.addEventListener("keydown", handleEscape, true);
  target.addEventListener("pointercancel", handlePointerCancel, true);
  target.addEventListener("touchcancel", handleTouchCancel, true);
  target.addEventListener("lostpointercapture", handleLostPointerCapture, true);

  return () => {
    target.removeEventListener("keydown", handleEscape, true);
    target.removeEventListener("pointercancel", handlePointerCancel, true);
    target.removeEventListener("touchcancel", handleTouchCancel, true);
    target.removeEventListener("lostpointercapture", handleLostPointerCapture, true);
  };
}
