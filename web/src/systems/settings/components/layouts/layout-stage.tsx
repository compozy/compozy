import { useState } from "react";

import type { WindowManagerConfig } from "@/systems/os";

import { useLayoutCanvasSelection } from "../../hooks/use-layout-canvas-selection";
import type { WindowManagerLayoutEditorModel } from "../../hooks/use-window-manager-layout-editor";
import { layoutCanvasProjection } from "../../lib/window-manager-layout-canvas";
import { LayoutCanvas } from "./layout-canvas";
import { LayoutCanvasBoardBar } from "./layout-canvas-board-bar";
import { LayoutDesktopTabs } from "./layout-desktop-tabs";
import { LayoutInspector } from "./layout-inspector";
import { LayoutReviewBar } from "./layout-review-bar";

interface LayoutStageProps {
  editor: WindowManagerLayoutEditorModel;
  config: WindowManagerConfig;
}

/**
 * The workspace layout as one object: the desktop you can grab, what the
 * selection is, and the daemon's check — all inside a single card, so the review
 * gate belongs visibly to the thing being reviewed.
 */
export function LayoutStage({ editor, config }: LayoutStageProps) {
  const [activeIndex, setActiveIndex] = useState(0);
  const document = editor.draft;
  const index = Math.min(activeIndex, Math.max(0, document.desktops.length - 1));
  const desktop = document.desktops[index];
  const selection = useLayoutCanvasSelection(
    desktop ?? {
      id: "",
      name: "",
      order: 0,
      purpose: "standard",
      focusOwner: null,
      groups: [],
      floating: [],
    }
  );

  if (!desktop) {
    return (
      <div
        className="rounded-lg border border-line bg-canvas-soft px-4 py-5 text-form-label text-subtle"
        data-testid="layout-stage-empty"
      >
        This workspace has no desktops yet. Open a window to create the first one.
      </div>
    );
  }

  const projection = layoutCanvasProjection(document, desktop, config, editor.revision);

  return (
    <div
      className="@container/stage overflow-hidden rounded-lg border border-line bg-canvas-soft"
      data-testid="layout-stage"
    >
      <LayoutDesktopTabs
        activeIndex={index}
        desktops={document.desktops}
        onRename={editor.setDesktopName}
        onSelect={position => {
          setActiveIndex(position);
          selection.clear();
        }}
      />

      <div className="grid @min-layout-stage-split/stage:grid-cols-[minmax(0,1fr)_var(--width-layout-inspector)]">
        <div className="flex min-w-0 flex-col gap-2 p-4">
          <LayoutCanvas
            config={config}
            desktop={desktop}
            document={document}
            projection={projection}
            selection={selection.selection}
            onDesktopChange={next => editor.setDesktop(index, next)}
            onFloatingRectChange={editor.setFloatingRect}
            onSelect={selection.select}
          />
          <LayoutCanvasBoardBar desktop={desktop} document={document} projection={projection} />
        </div>
        <div className="border-t border-line-soft @min-layout-stage-split/stage:border-t-0 @min-layout-stage-split/stage:border-l">
          <LayoutInspector
            desktop={desktop}
            document={document}
            selection={selection.selection}
            onDesktopChange={next => editor.setDesktop(index, next)}
          />
        </div>
      </div>

      <LayoutReviewBar editor={editor} />
    </div>
  );
}
