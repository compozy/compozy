import { ReactFlowProvider } from "@xyflow/react";

import { cn, LaneTabs, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { useLoopEditorChrome } from "../../hooks/use-loop-editor-chrome";
import { placeholderSlots } from "../../lib/placeholder-slots";
import {
  SIDEBAR_CLASS,
  SIDEBAR_TABS,
  SIDEBAR_TABS_CLASS,
  SIDEBAR_TABS_LIST_CLASS,
} from "./loop-editor-chrome-parts";
import { editorGridClass } from "./loop-editor-grid";
import { LoopEditorPalette } from "./loop-editor-palette";
import { LoopEditorCanvasSkeleton } from "./loop-editor-skeleton-parts";
import { LoopEditorToolbar } from "./loop-editor-toolbar";

const noop = () => undefined;

function SidebarSkeleton() {
  return (
    <aside className={SIDEBAR_CLASS}>
      <LaneTabs
        ariaLabel="Loop editor inspector"
        className={SIDEBAR_TABS_CLASS}
        items={SIDEBAR_TABS}
        listClassName={SIDEBAR_TABS_LIST_CLASS}
        onChange={noop}
        value="contract"
      >
        <div className="min-h-0 flex-1 overflow-y-auto">
          <section className="bg-canvas px-4 py-3.5">
            <p className="mb-3 text-form-hint leading-relaxed text-subtle">
              Define the outcome this Loop should reach.
            </p>
            <div className="flex flex-col gap-3.5">
              {placeholderSlots("field", 2).map(slot => (
                <div className="flex flex-col gap-1.5" key={slot.id}>
                  <SkeletonText className="w-24 text-form-label" />
                  <Skeleton className="h-16 w-full" />
                </div>
              ))}
            </div>
          </section>
        </div>
      </LaneTabs>
    </aside>
  );
}

/**
 * The editor before the Loop has been read: the editor's own chrome (toolbar, node palette,
 * inspector tabs and validation bar), which does not depend on the Loop, with the canvas and the
 * contract fields as placeholders. The chrome is the real components made inert, so it cannot
 * drift from the ready editor and nothing in it can be used while the Loop is unread.
 */
export function LoopEditorSkeleton() {
  const { paletteMode, inspectorOpen } = useLoopEditorChrome();
  return (
    <SkeletonRegion
      className="flex min-h-0 flex-1 flex-col"
      data-testid="loop-editor-loading"
      label="Loading Loop editor"
    >
      <div className="flex min-h-0 flex-1 flex-col gap-0" inert>
        <ReactFlowProvider>
          <LoopEditorToolbar
            addNodeDisabled
            busy
            inspectorOpen={inspectorOpen}
            onAddNode={noop}
            onAutoLayout={noop}
            onSaveLayout={noop}
            onToggleInspector={noop}
            onTogglePalette={noop}
            onViewChange={noop}
            paletteMode={paletteMode}
            positionsDirty={false}
            view="graph"
          />
        </ReactFlowProvider>
        <div className={cn("grid min-h-0 flex-1", editorGridClass(paletteMode, inspectorOpen))}>
          {paletteMode === "expanded" ? <LoopEditorPalette disabled onAddNode={noop} /> : null}
          <LoopEditorCanvasSkeleton />
          {inspectorOpen ? <SidebarSkeleton /> : null}
        </div>
      </div>
    </SkeletonRegion>
  );
}
