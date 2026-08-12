import { cn } from "@compozy/ui";

import { useFrameSeam } from "../hooks/use-frame-seam";
import { useLayoutSeam } from "../hooks/use-layout-seam";
import type {
  LayoutProjection,
  ProjectedFrameSeam,
  ProjectedSeam,
} from "../lib/window-manager-types";
import { WINDOW_VISUAL_LAYER } from "../lib/window-visual-layer";

export type SeamGestureHandlers = {
  onResize: (splitId: string, boundaryIndex: number, delta: number) => void;
  onFrameResize: (seam: ProjectedFrameSeam, deltaPx: number) => void;
  onSeamPreview: (seam: ProjectedSeam, deltaPx: number) => void;
  onFrameSeamPreview: (seam: ProjectedFrameSeam, deltaPx: number) => void;
  onSeamPreviewEnd: () => void;
};

const SEAM_CLASS_BASE = cn(
  "absolute touch-none rounded-pill outline-none",
  "before:absolute before:rounded-pill before:bg-line-strong",
  "hover:before:bg-accent focus-visible:shadow-focus-ring focus-visible:before:bg-accent"
);

function seamClassName(vertical: boolean, dragging: boolean): string {
  return cn(
    SEAM_CLASS_BASE,
    dragging && "before:bg-accent",
    vertical
      ? "w-3 cursor-col-resize before:top-0 before:bottom-0 before:left-1/2 before:w-px before:-translate-x-1/2"
      : "h-3 cursor-row-resize before:top-1/2 before:right-0 before:left-0 before:h-px before:-translate-y-1/2"
  );
}

function seamStyle(vertical: boolean, rect: ProjectedSeam["rect"]) {
  return {
    left: rect.x - (vertical ? 6 : 0),
    top: rect.y - (vertical ? 0 : 6),
    width: vertical ? 12 : rect.w,
    height: vertical ? rect.h : 12,
    zIndex: WINDOW_VISUAL_LAYER.seam,
  };
}

function LayoutSeam({
  seam,
  onResize,
  onSeamPreview,
  onSeamPreviewEnd,
}: {
  seam: ProjectedSeam;
} & Pick<SeamGestureHandlers, "onResize" | "onSeamPreview" | "onSeamPreviewEnd">) {
  const model = useLayoutSeam(seam, onResize, onSeamPreview, onSeamPreviewEnd);
  const vertical = seam.orientation === "vertical";

  return (
    <div
      role="separator"
      tabIndex={0}
      aria-label={`Resize boundary ${seam.boundaryIndex + 1}`}
      aria-orientation={vertical ? "vertical" : "horizontal"}
      aria-valuemin={Math.round(seam.minValue)}
      aria-valuemax={Math.round(seam.maxValue)}
      aria-valuenow={Math.round(seam.value)}
      data-split-id={seam.splitId}
      data-boundary-index={seam.boundaryIndex}
      className={seamClassName(vertical, model.dragging)}
      style={seamStyle(vertical, seam.rect)}
      onPointerDown={model.handlePointerDown}
      onPointerMove={model.handlePointerMove}
      onPointerCancel={model.handlePointerCancel}
      onLostPointerCapture={model.handleLostPointerCapture}
      onPointerUp={model.handlePointerUp}
      onKeyDown={model.handleKeyDown}
    />
  );
}

function FrameSeam({
  seam,
  position,
  count,
  onFrameResize,
  onFrameSeamPreview,
  onSeamPreviewEnd,
}: {
  seam: ProjectedFrameSeam;
  position: number;
  count: number;
} & Pick<SeamGestureHandlers, "onFrameResize" | "onFrameSeamPreview" | "onSeamPreviewEnd">) {
  const model = useFrameSeam(seam, onFrameResize, onFrameSeamPreview, onSeamPreviewEnd);
  const vertical = seam.orientation === "vertical";

  return (
    <div
      role="separator"
      tabIndex={0}
      aria-label={`Resize island boundary ${position} of ${count}`}
      aria-orientation={vertical ? "vertical" : "horizontal"}
      aria-valuemin={Math.round(seam.minValue)}
      aria-valuemax={Math.round(seam.maxValue)}
      aria-valuenow={Math.round(seam.value)}
      data-frame-seam-id={seam.id}
      className={seamClassName(vertical, model.dragging)}
      style={seamStyle(vertical, seam.rect)}
      onPointerDown={model.handlePointerDown}
      onPointerMove={model.handlePointerMove}
      onPointerCancel={model.handlePointerCancel}
      onLostPointerCapture={model.handleLostPointerCapture}
      onPointerUp={model.handlePointerUp}
      onKeyDown={model.handleKeyDown}
    />
  );
}

/** Structural split seams plus shared island boundaries, one draggable layer. */
export function OsSnapSeamLayer({
  projection,
  onResize,
  onFrameResize,
  onSeamPreview,
  onFrameSeamPreview,
  onSeamPreviewEnd,
}: { projection: LayoutProjection | undefined } & SeamGestureHandlers) {
  if (!projection) return null;
  return (
    <>
      {projection.seams.map(seam => (
        <LayoutSeam
          key={seam.id}
          seam={seam}
          onResize={onResize}
          onSeamPreview={onSeamPreview}
          onSeamPreviewEnd={onSeamPreviewEnd}
        />
      ))}
      {projection.frameSeams.map((seam, index) => (
        <FrameSeam
          key={seam.id}
          seam={seam}
          position={index + 1}
          count={projection.frameSeams.length}
          onFrameResize={onFrameResize}
          onFrameSeamPreview={onFrameSeamPreview}
          onSeamPreviewEnd={onSeamPreviewEnd}
        />
      ))}
    </>
  );
}
