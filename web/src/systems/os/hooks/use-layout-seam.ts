import { useEffect, useEffectEvent, type KeyboardEvent, type PointerEvent } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { seamWeightDelta } from "../lib/seam-preview";
import type { ProjectedSeam } from "../lib/window-manager-types";
import { createSeamDragLogic } from "./seam-drag-store";

/** Matches the daemon's `weightTolerance` no-op band for layout.resize. */
const SEAM_WEIGHT_EPSILON = 0.000001;

function resizeKeyDelta(event: KeyboardEvent<HTMLElement>): number | null {
  if (event.key === "ArrowLeft" || event.key === "ArrowUp") return -0.02;
  if (event.key === "ArrowRight" || event.key === "ArrowDown") return 0.02;
  return null;
}

const layoutSeamDragLogic = createSeamDragLogic<ProjectedSeam>();

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
  const dragStore = useStore(layoutSeamDragLogic);
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
        preview: (capturedSeam, deltaPx) => onSeamPreview(capturedSeam, deltaPx),
      });
    },
    handlePointerUp: event => {
      const state = dragStore.getSnapshot().context;
      if (state.phase !== "dragging" || state.pointerId !== event.pointerId || !state.seam) return;
      dragStore.trigger.dragEnded({ pointerId: event.pointerId });
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
      if (dragStore.getSnapshot().context.phase === "dragging") cancelDrag();
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
