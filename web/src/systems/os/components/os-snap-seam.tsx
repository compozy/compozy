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

/**
 * Seam grammar (shell-rail prototype `.seam`): a 9px hit strip centred on the
 * boundary whose 1px `line` hairline is the only divider between flush panes;
 * hover, drag and keyboard focus widen it to 2px `accent`.
 */
const SEAM_CLASS_BASE = cn(
  "absolute touch-none outline-none",
  "before:absolute before:bg-line before:transition-colors before:duration-fast",
  "hover:before:bg-accent focus-visible:before:bg-accent"
);

function seamClassName(vertical: boolean, dragging: boolean): string {
  return cn(
    SEAM_CLASS_BASE,
    dragging && "before:bg-accent",
    vertical
      ? [
          "cursor-col-resize before:inset-y-0 before:left-1/2 before:w-px before:-translate-x-1/2",
          "hover:before:w-0.5 focus-visible:before:w-0.5",
          dragging && "before:w-0.5",
        ]
      : [
          "cursor-row-resize before:inset-x-0 before:top-1/2 before:h-px before:-translate-y-1/2",
          "hover:before:h-0.5 focus-visible:before:h-0.5",
          dragging && "before:h-0.5",
        ]
  );
}

const SEAM_HIT = "var(--size-seam-hit)";

function seamStyle(vertical: boolean, rect: ProjectedSeam["rect"]) {
  return {
    left: vertical ? `calc(${rect.x}px - ${SEAM_HIT} / 2)` : rect.x,
    top: vertical ? rect.y : `calc(${rect.y}px - ${SEAM_HIT} / 2)`,
    width: vertical ? SEAM_HIT : rect.w,
    height: vertical ? rect.h : SEAM_HIT,
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
