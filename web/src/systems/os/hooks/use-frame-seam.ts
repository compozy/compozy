import { useEffect, useEffectEvent, type KeyboardEvent, type PointerEvent } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { frameSeamDeltaNormalized } from "../lib/frame-seams";
import type { ProjectedFrameSeam } from "../lib/window-manager-types";
import { createSeamDragLogic } from "./seam-drag-store";

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

const frameSeamDragLogic = createSeamDragLogic<ProjectedFrameSeam>();

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
  const dragStore = useStore(frameSeamDragLogic);
  const dragging = useSelector(dragStore, snapshot => snapshot.context.phase === "dragging");
  const vertical = seam.orientation === "vertical";

  const cancelDrag = () => {
    dragStore.trigger.dragCancelled({ endPreview: onSeamPreviewEnd });
  };

  const handleEscapeKeyDown = useEffectEvent((event: globalThis.KeyboardEvent) => {
    if (event.key !== "Escape") return;
    dragStore.trigger.dragCancelled({ endPreview: onSeamPreviewEnd });
  });

  useEffect(() => {
    if (!dragging) return;
    const handleKeyDown = (event: globalThis.KeyboardEvent) => handleEscapeKeyDown(event);
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [dragging]);

  const cleanupActivePreview = useEffectEvent(() =>
    dragStore.trigger.disposed({ endPreview: onSeamPreviewEnd })
  );

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
      dragStore.trigger.dragStarted({
        pointerId: event.pointerId,
        coordinate: vertical ? event.clientX : event.clientY,
        seam,
      });
    },
    handlePointerMove: event => {
      dragStore.trigger.pointerMoved({
        coordinate: vertical ? event.clientX : event.clientY,
        pointerId: event.pointerId,
        preview: (capturedSeam, deltaPx) => onFrameSeamPreview(capturedSeam, deltaPx),
      });
    },
    handlePointerUp: event => {
      const state = dragStore.getSnapshot().context;
      if (state.phase !== "dragging" || state.pointerId !== event.pointerId || !state.seam) return;
      dragStore.trigger.dragEnded({ pointerId: event.pointerId });
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
      if (dragStore.getSnapshot().context.phase === "dragging") cancelDrag();
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
