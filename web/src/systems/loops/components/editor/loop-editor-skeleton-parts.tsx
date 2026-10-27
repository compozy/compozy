import { cn, Eyebrow, Skeleton, SkeletonRegion } from "@compozy/ui";

import { DOCK_BAR_CLASS, DOCK_CLASS, EDITOR_TOOLBAR_CLASS } from "./loop-editor-chrome-parts";
import { editorGridClass } from "./loop-editor-grid";

// The parts of the editor's loading shapes that need no graph library, so a window can show them
// before the editor's code has loaded. The skeleton that wears the real toolbar and palette lives
// beside them and brings the graph library with it.

/** Step cards on the canvas as a Loop of a few steps lays them out; decorative, so no shift. */
const CANVAS_STEPS = [
  { id: "step-a", className: "top-16 left-16" },
  { id: "step-b", className: "top-44 left-72" },
  { id: "step-c", className: "top-72 left-28" },
] as const;

/** The canvas as placeholder step cards over the validation dock's always-visible bar. */
export function LoopEditorCanvasSkeleton() {
  return (
    <section className="relative flex min-h-0 flex-col bg-canvas">
      <div aria-hidden="true" className="relative min-h-0 flex-1">
        {CANVAS_STEPS.map(step => (
          <Skeleton className={cn("absolute h-24 w-44", step.className)} key={step.id} />
        ))}
      </div>
      <div className={DOCK_CLASS}>
        <div className={DOCK_BAR_CLASS}>
          <Eyebrow className="text-subtle">Validation</Eyebrow>
          <Skeleton className="h-pill-xs w-16 rounded-pill" />
        </div>
      </div>
    </section>
  );
}

/**
 * The editor before its code has loaded: the toolbar row as blocks over the canvas and dock, in
 * the editor's default chrome (palette and inspector closed, which is a person's own choice to
 * change and is not known yet). One status region; nothing in it takes focus.
 */
export function LoopEditorStaticSkeleton() {
  return (
    <SkeletonRegion
      className="flex min-h-0 flex-1 flex-col"
      data-testid="loop-editor-loading"
      label="Loading Loop editor"
    >
      <div className="flex min-h-0 flex-1 flex-col gap-0">
        <div aria-hidden="true" className={EDITOR_TOOLBAR_CLASS}>
          <Skeleton className="size-button-icon-sm rounded-md" />
          <Skeleton className="h-control-compact w-28 rounded-pill" />
          <Skeleton className="ml-auto h-control-compact w-48 rounded-md" />
          <Skeleton className="h-button-sm w-20 rounded-pill" />
        </div>
        <div className={cn("grid min-h-0 flex-1", editorGridClass("collapsed", false))}>
          <LoopEditorCanvasSkeleton />
        </div>
      </div>
    </SkeletonRegion>
  );
}
