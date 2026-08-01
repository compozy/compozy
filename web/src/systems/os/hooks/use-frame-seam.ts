import {
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";

import { frameSeamDeltaNormalized } from "../lib/frame-seams";
import type { ProjectedFrameSeam } from "../lib/window-manager-types";

/** Matches the daemon's `weightTolerance` no-op band. */
const FRAME_SEAM_EPSILON = 0.000001;

function resizeKeyStep(
  event: KeyboardEvent<HTMLElement>,
  orientation: ProjectedFrameSeam["orientation"]
): number | null {
  if (orientation === "vertical" && event.key === "ArrowLeft") return -0.02;
  if (orientation === "vertical" && event.key === "ArrowRight") return 0.02;
  if (orientation === "horizontal" && event.key === "ArrowUp") return -0.02;
  if (orientation === "horizontal" && event.key === "ArrowDown") return 0.02;
  return null;
}

interface FrameSeamDrag {
  pointerId: number;
  coordinate: number;
  /** Seam captured at pointer-down so the live preview never compounds itself. */
  seam: ProjectedFrameSeam;
}

export interface FrameSeamModel {
  dragging: boolean;
  handlePointerDown: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerMove: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerUp: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerCancel: () => void;
  handleLostPointerCapture: () => void;
  handleKeyDown: (event: KeyboardEvent<HTMLElement>) => void;
}

/** Live island-boundary drag: rAF-throttled preview, Escape cleanup, one commit. */
export function useFrameSeam(
  seam: ProjectedFrameSeam,
  onFrameResize: (seam: ProjectedFrameSeam, deltaPx: number) => void,
  onFrameSeamPreview: (seam: ProjectedFrameSeam, deltaPx: number) => void,
  onSeamPreviewEnd: () => void
): FrameSeamModel {
  const drag = useRef<FrameSeamDrag | null>(null);
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
      // Suppresses the browser's native drag-select under the moving pointer.
      event.preventDefault();
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
        if (active !== null) onFrameSeamPreview(active.seam, pendingDeltaPx.current);
      });
    },
    handlePointerUp: event => {
      const state = drag.current;
      if (!state || state.pointerId !== event.pointerId) return;
      cancelFrame();
      drag.current = null;
      setDragging(false);
      const deltaPixels = (vertical ? event.clientX : event.clientY) - state.coordinate;
      if (Math.abs(frameSeamDeltaNormalized(state.seam, deltaPixels)) > FRAME_SEAM_EPSILON) {
        // The shell clears the preview once the resize command reconciles, so
        // the islands never flash back to their pre-drag sizes.
        onFrameResize(state.seam, deltaPixels);
        return;
      }
      onSeamPreviewEnd();
    },
    handlePointerCancel: cancelDrag,
    handleLostPointerCapture: () => {
      if (drag.current !== null) cancelDrag();
    },
    handleKeyDown: event => {
      const step = resizeKeyStep(event, seam.orientation);
      if (step === null) return;
      event.preventDefault();
      const deltaPx = step * seam.axisSpan;
      if (Math.abs(frameSeamDeltaNormalized(seam, deltaPx)) > FRAME_SEAM_EPSILON) {
        onFrameResize(seam, deltaPx);
      }
    },
  };
}
