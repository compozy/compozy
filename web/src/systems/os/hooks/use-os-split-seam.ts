import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";

import { seamWeightDelta } from "../lib/seam-preview";
import type { ProjectedSeam } from "../lib/window-manager-types";
import { windowManagerStore } from "../stores/window-manager-store";
import { useOsShell } from "./use-os-shell";

/** Matches the daemon's no-op band for `layout.resize`. */
const SEAM_WEIGHT_EPSILON = 0.000001;

/** Keyboard step in weight space: 2% per arrow, 10% with Shift. */
export function seamKeyStep(event: { shiftKey: boolean }): number {
  return event.shiftKey ? 0.1 : 0.02;
}

function keyDirection(key: string): -1 | 1 | null {
  if (key === "ArrowLeft" || key === "ArrowUp") return -1;
  if (key === "ArrowRight" || key === "ArrowDown") return 1;
  return null;
}

/** A pixel drag pushes past a member's minimum: the seam stops there and says so. */
function atLimit(seam: ProjectedSeam, deltaPx: number): boolean {
  const value = seam.value + deltaPx;
  return value < seam.minValue || value > seam.maxValue;
}

function clearPreview(): void {
  windowManagerStore.trigger.seamPreviewCleared();
}

export type OsSplitSeamState = "drag" | "limit" | undefined;

/**
 * One seam's gesture: a pointer drag previews the two sides' shares through the shared seam
 * preview, stops at either side's pixel minimum, and commits one `layout.resize`; Escape restores
 * the shares. Arrow keys resize in steps and a double-click balances the two sides.
 */
export function useOsSplitSeam(seam: ProjectedSeam | undefined, memberCount: number) {
  const { manager } = useOsShell();
  const [state, setState] = useState<OsSplitSeamState>(undefined);
  const stopRef = useRef<(() => void) | null>(null);
  const vertical = seam?.orientation === "vertical";

  // A remote commit can unmount the seam mid-drag; never leave a stale preview behind.
  useEffect(() => () => stopRef.current?.(), []);

  const commit = (captured: ProjectedSeam, delta: number) => {
    if (Math.abs(delta) <= SEAM_WEIGHT_EPSILON) {
      clearPreview();
      return;
    }
    // The preview stays until the command settles, so the panes never flash back.
    void manager
      .resizeLayout(captured.splitId, captured.boundaryIndex, delta)
      .completion.finally(clearPreview);
  };

  const onPointerDown = (event: PointerEvent<HTMLElement>) => {
    if (seam === undefined || event.button !== 0 || stopRef.current !== null) return;
    // Suppresses the browser's text selection under the moving pointer.
    event.preventDefault();
    const element = event.currentTarget;
    const pointerId = event.pointerId;
    const captured = seam;
    const origin = vertical ? event.clientX : event.clientY;
    let deltaPx = 0;
    let frame: number | null = null;
    element.setPointerCapture(pointerId);
    setState("drag");

    const stop = () => {
      stopRef.current = null;
      if (frame !== null) cancelAnimationFrame(frame);
      window.removeEventListener("keydown", onKey, true);
      element.removeEventListener("pointermove", onMove);
      element.removeEventListener("pointerup", onUp);
      element.removeEventListener("pointercancel", onCancel);
      element.removeEventListener("lostpointercapture", onCancel);
      if (element.hasPointerCapture(pointerId)) element.releasePointerCapture(pointerId);
      setState(undefined);
    };
    const onMove = (move: globalThis.PointerEvent) => {
      if (move.pointerId !== pointerId) return;
      deltaPx = (vertical ? move.clientX : move.clientY) - origin;
      setState(atLimit(captured, deltaPx) ? "limit" : "drag");
      if (frame !== null) return;
      frame = requestAnimationFrame(() => {
        frame = null;
        windowManagerStore.trigger.seamPreviewSet({
          preview: { splitId: captured.splitId, boundaryIndex: captured.boundaryIndex, deltaPx },
        });
      });
    };
    const onUp = (up: globalThis.PointerEvent) => {
      if (up.pointerId !== pointerId) return;
      deltaPx = (vertical ? up.clientX : up.clientY) - origin;
      stop();
      commit(captured, seamWeightDelta(captured, deltaPx));
    };
    const onCancel = () => {
      stop();
      clearPreview();
    };
    const onKey = (key: globalThis.KeyboardEvent) => {
      if (key.key !== "Escape") return;
      key.preventDefault();
      key.stopPropagation();
      onCancel();
    };
    stopRef.current = onCancel;
    window.addEventListener("keydown", onKey, true);
    element.addEventListener("pointermove", onMove);
    element.addEventListener("pointerup", onUp);
    element.addEventListener("pointercancel", onCancel);
    element.addEventListener("lostpointercapture", onCancel);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (seam === undefined) return;
    const direction = keyDirection(event.key);
    if (direction === null) return;
    event.preventDefault();
    const delta = seamWeightDelta(seam, direction * seamKeyStep(event) * seam.axisSpan);
    if (Math.abs(delta) > SEAM_WEIGHT_EPSILON) {
      manager.resizeLayout(seam.splitId, seam.boundaryIndex, delta);
    }
  };

  /**
   * Balances the two sides this seam separates. With two members that is the whole split, which
   * `layout.balance` evens; in a wider split only this pair is evened, so the others keep theirs.
   */
  const onDoubleClick = () => {
    if (seam === undefined) return;
    if (memberCount <= 2) {
      manager.balanceLayout({ splitId: seam.splitId });
      return;
    }
    const delta = (seam.trailingWeight - seam.leadingWeight) / 2;
    if (Math.abs(delta) > SEAM_WEIGHT_EPSILON) {
      manager.resizeLayout(seam.splitId, seam.boundaryIndex, delta);
    }
  };

  return { state, onPointerDown, onKeyDown, onDoubleClick };
}
