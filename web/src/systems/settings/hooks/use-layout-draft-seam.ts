import { useState, type KeyboardEvent as ReactKeyboardEvent, type RefObject } from "react";

import { applySeamPreviewToDesktop, seamWeightDelta, type ProjectedSeam } from "@/systems/os";

import { toDraftDesktop } from "../lib/window-manager-layout-bridge";
import { LAYOUT_CANVAS_REFERENCE } from "../lib/window-manager-layout-reference";
import { fractionLabel, nearestDetent } from "../lib/window-manager-layout-detents";
import type { WindowManagerLayoutDesktop } from "../lib/window-manager-layout-types";
import { usePointerDrag } from "./use-pointer-drag";

/** Matches the live shell's keyboard step so both surfaces nudge by the same amount. */
const KEYBOARD_STEP = 0.02;

interface SeamDragState {
  desktop: WindowManagerLayoutDesktop;
  origin: number;
  scale: number;
  /** The projection at pointer-down; parent re-projection must not compound the gesture. */
  seam: ProjectedSeam;
}

export interface LayoutDraftSeamModel {
  dragging: boolean;
  /** Leading share of the pair, 0–1, after clamping and detents. */
  ratio: number;
  leadingLabel: string;
  trailingLabel: string;
  snapped: boolean;
  onPointerDown: (event: React.PointerEvent<HTMLElement>) => void;
  onKeyDown: (event: ReactKeyboardEvent<HTMLElement>) => void;
}

/**
 * Drags one divider on the *draft*, not the running workspace.
 *
 * The pixel-to-weight conversion and the immutable rebalance are the runtime's
 * own (`seam-preview.ts`, a mirror of `reducer_layout_resize.go`), which is why
 * a drag here cannot produce the weight vector the daemon rejects: the split
 * still sums to 1 by construction rather than by correction.
 */
export function useLayoutDraftSeam(
  seam: ProjectedSeam,
  desktop: WindowManagerLayoutDesktop,
  canvasRef: RefObject<HTMLElement | null>,
  onChange: (next: WindowManagerLayoutDesktop) => void
): LayoutDraftSeamModel {
  const [preview, setPreview] = useState<{ ratio: number; snapped: boolean } | null>(null);
  const pairTotal = seam.leadingWeight + seam.trailingWeight;
  const restingRatio = pairTotal === 0 ? 0.5 : seam.leadingWeight / pairTotal;

  const commit = (state: SeamDragState, deltaPixels: number) => {
    const clampedWeightDelta = seamWeightDelta(state.seam, deltaPixels);
    const pairTotal = state.seam.leadingWeight + state.seam.trailingWeight;
    const proposed = state.seam.leadingWeight + clampedWeightDelta;
    const proposedRatio = pairTotal === 0 ? 0.5 : proposed / pairTotal;
    const detent = nearestDetent(proposedRatio);
    const targetLeading = detent === null ? proposed : detent * pairTotal;
    const snappedDelta = (targetLeading - state.seam.leadingWeight) * state.seam.axisSpan;
    setPreview({
      ratio: pairTotal === 0 ? 0.5 : targetLeading / pairTotal,
      snapped: detent !== null,
    });
    onChange(toDraftDesktop(applySeamPreviewToDesktop(state.desktop, state.seam, snappedDelta)));
  };

  const drag = usePointerDrag<SeamDragState>({
    onStart: event => {
      const canvas = canvasRef.current?.getBoundingClientRect();
      if (!canvas || canvas.width === 0 || canvas.height === 0) return null;
      const vertical = seam.orientation === "vertical";
      return {
        desktop,
        origin: vertical ? event.clientX : event.clientY,
        seam,
        // The canvas is a scale model of the reference screen, so one CSS pixel
        // of travel is this many reference pixels.
        scale: vertical
          ? LAYOUT_CANVAS_REFERENCE.w / canvas.width
          : LAYOUT_CANVAS_REFERENCE.h / canvas.height,
      };
    },
    onMove: (event, state) => {
      const pointer = seam.orientation === "vertical" ? event.clientX : event.clientY;
      commit(state, (pointer - state.origin) * state.scale);
    },
    onEnd: () => setPreview(null),
  });

  const onKeyDown = (event: ReactKeyboardEvent<HTMLElement>) => {
    const forward = event.key === "ArrowRight" || event.key === "ArrowDown";
    const backward = event.key === "ArrowLeft" || event.key === "ArrowUp";
    if (!forward && !backward) return;
    event.preventDefault();
    event.stopPropagation();
    const step = (forward ? KEYBOARD_STEP : -KEYBOARD_STEP) * seam.axisSpan;
    onChange(toDraftDesktop(applySeamPreviewToDesktop(desktop, seam, step)));
  };

  const ratio = drag.dragging && preview ? preview.ratio : restingRatio;

  return {
    dragging: drag.dragging,
    ratio,
    leadingLabel: fractionLabel(ratio),
    trailingLabel: fractionLabel(1 - ratio),
    snapped: drag.dragging && (preview?.snapped ?? false),
    onPointerDown: drag.onPointerDown,
    onKeyDown,
  };
}
