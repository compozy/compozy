import {
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";

import { seamWeightDelta } from "../lib/seam-preview";
import type { ProjectedSeam } from "../lib/window-manager-types";

/** Matches the daemon's `weightTolerance` no-op band for layout.resize. */
const SEAM_WEIGHT_EPSILON = 0.000001;

function resizeKeyDelta(event: KeyboardEvent<HTMLElement>): number | null {
  if (event.key === "ArrowLeft" || event.key === "ArrowUp") return -0.02;
  if (event.key === "ArrowRight" || event.key === "ArrowDown") return 0.02;
  return null;
}

interface SeamDrag {
  pointerId: number;
  coordinate: number;
  /** Seam captured at pointer-down: preview and commit both resolve against
   * the unadjusted projection, so the live preview never compounds itself. */
  seam: ProjectedSeam;
}

export interface LayoutSeamModel {
  dragging: boolean;
  handlePointerDown: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerMove: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerUp: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerCancel: () => void;
  handleLostPointerCapture: () => void;
  handleKeyDown: (event: KeyboardEvent<HTMLElement>) => void;
}

/** Live seam drag: rAF-throttled preview, Escape/cancel cleanup, one commit. */
export function useLayoutSeam(
  seam: ProjectedSeam,
  onResize: (splitId: string, boundaryIndex: number, delta: number) => void,
  onSeamPreview: (seam: ProjectedSeam, deltaPx: number) => void,
  onSeamPreviewEnd: () => void
): LayoutSeamModel {
  const drag = useRef<SeamDrag | null>(null);
  const frame = useRef<number | null>(null);
  const pendingDeltaPx = useRef(0);
  const [dragging, setDragging] = useState(false);
  const vertical = seam.orientation === "vertical";

  const cancelFrame = () => {
    if (frame.current !== null) {
      cancelAnimationFrame(frame.current);
      frame.current = null;
    }
  };

  const cancelDrag = () => {
    cancelFrame();
    drag.current = null;
    setDragging(false);
    onSeamPreviewEnd();
  };

  const handleEscapeKeyDown = useEffectEvent((event: globalThis.KeyboardEvent) => {
    if (event.key !== "Escape") return;
    cancelFrame();
    drag.current = null;
    setDragging(false);
    onSeamPreviewEnd();
  });

  useEffect(() => {
    if (!dragging) return;
    const handleKeyDown = (event: globalThis.KeyboardEvent) => handleEscapeKeyDown(event);
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [dragging]);

  const cleanupActivePreview = useEffectEvent(() => {
    if (frame.current !== null) cancelAnimationFrame(frame.current);
    if (drag.current !== null) onSeamPreviewEnd();
  });

  // A remote commit can unmount the seam mid-drag; never leave a stale preview.
  useEffect(() => {
    return () => cleanupActivePreview();
  }, []);

  return {
    dragging,
    handlePointerDown: event => {
      if (event.button !== 0) return;
      event.currentTarget.setPointerCapture(event.pointerId);
      drag.current = {
        pointerId: event.pointerId,
        coordinate: vertical ? event.clientX : event.clientY,
        seam,
      };
      pendingDeltaPx.current = 0;
      setDragging(true);
    },
    handlePointerMove: event => {
      const state = drag.current;
      if (!state || state.pointerId !== event.pointerId) return;
      pendingDeltaPx.current = (vertical ? event.clientX : event.clientY) - state.coordinate;
      if (frame.current !== null) return;
      frame.current = requestAnimationFrame(() => {
        frame.current = null;
        const active = drag.current;
        if (active !== null) onSeamPreview(active.seam, pendingDeltaPx.current);
      });
    },
    handlePointerUp: event => {
      const state = drag.current;
      if (!state || state.pointerId !== event.pointerId) return;
      cancelFrame();
      drag.current = null;
      setDragging(false);
      const deltaPixels = (vertical ? event.clientX : event.clientY) - state.coordinate;
      const delta = seamWeightDelta(state.seam, deltaPixels);
      if (Math.abs(delta) > SEAM_WEIGHT_EPSILON) {
        // The shell clears the preview once the resize command reconciles, so
        // the panes never flash back to their pre-drag sizes.
        onResize(state.seam.splitId, state.seam.boundaryIndex, delta);
        return;
      }
      onSeamPreviewEnd();
    },
    handlePointerCancel: cancelDrag,
    handleLostPointerCapture: () => {
      if (drag.current !== null) cancelDrag();
    },
    handleKeyDown: event => {
      const weightStep = resizeKeyDelta(event);
      if (weightStep === null) return;
      event.preventDefault();
      const delta = seamWeightDelta(seam, weightStep * seam.axisSpan);
      if (Math.abs(delta) > SEAM_WEIGHT_EPSILON) {
        onResize(seam.splitId, seam.boundaryIndex, delta);
      }
    },
  };
}
