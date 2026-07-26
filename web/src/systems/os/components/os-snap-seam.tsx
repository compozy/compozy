import { cn } from "@agh/ui";

import { useLayoutSeam } from "../hooks/use-layout-seam";
import type { LayoutProjection, ProjectedSeam } from "../lib/window-manager-types";

export type SeamGestureHandlers = {
  onResize: (splitId: string, boundaryIndex: number, delta: number) => void;
  onSeamPreview: (seam: ProjectedSeam, deltaPx: number) => void;
  onSeamPreviewEnd: () => void;
};

function LayoutSeam({
  seam,
  onResize,
  onSeamPreview,
  onSeamPreviewEnd,
}: { seam: ProjectedSeam } & SeamGestureHandlers) {
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
      className={cn(
        "absolute z-30 touch-none rounded-pill outline-none",
        "before:absolute before:rounded-pill before:bg-line-strong",
        "hover:before:bg-accent focus-visible:shadow-focus-ring focus-visible:before:bg-accent",
        model.dragging && "before:bg-accent",
        vertical
          ? "w-3 cursor-col-resize before:top-0 before:bottom-0 before:left-1/2 before:w-px before:-translate-x-1/2"
          : "h-3 cursor-row-resize before:top-1/2 before:right-0 before:left-0 before:h-px before:-translate-y-1/2"
      )}
      style={{
        left: seam.rect.x - (vertical ? 6 : 0),
        top: seam.rect.y - (vertical ? 0 : 6),
        width: vertical ? 12 : seam.rect.w,
        height: vertical ? seam.rect.h : 12,
      }}
      onPointerDown={model.handlePointerDown}
      onPointerMove={model.handlePointerMove}
      onPointerCancel={model.handlePointerCancel}
      onLostPointerCapture={model.handleLostPointerCapture}
      onPointerUp={model.handlePointerUp}
      onKeyDown={model.handleKeyDown}
    />
  );
}

/** Structural seams come directly from split IDs and boundary indexes. */
export function OsSnapSeamLayer({
  projection,
  onResize,
  onSeamPreview,
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
    </>
  );
}
