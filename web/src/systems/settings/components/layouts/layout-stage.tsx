import { useState } from "react";

import { useLayoutCanvasSelection } from "../../hooks/use-layout-canvas-selection";
import type { WindowManagerLayoutEditorModel } from "../../hooks/use-window-manager-layout-editor";
import {
  renameLayoutDesktop,
  replaceLayoutTabRoot,
} from "../../lib/window-manager-layout-mutations";
import { LayoutCanvas } from "./layout-canvas";
import { LayoutDesktopTabs } from "./layout-desktop-tabs";
import { LayoutInspector } from "./layout-inspector";
import { LayoutReviewBar } from "./layout-review-bar";
import { LayoutTabStrip } from "./layout-tab-strip";

interface LayoutStageProps {
  editor: WindowManagerLayoutEditorModel;
}

/**
 * The workspace layout as one object: desktops, the picked desktop's tabs, the
 * picked tab's splits, what the selection can do, and the daemon's check — all
 * inside a single card, so the review gate belongs visibly to the thing being
 * reviewed.
 */
export function LayoutStage({ editor }: LayoutStageProps) {
  const [activeIndex, setActiveIndex] = useState(0);
  const document = editor.draft;
  const index = Math.min(activeIndex, Math.max(0, document.desktops.length - 1));
  const desktop = document.desktops[index];
  const selection = useLayoutCanvasSelection(desktop);
  const tab = selection.tab;

  if (!desktop || tab === null) {
    return (
      <div
        className="rounded-lg bg-card shadow-card px-4 py-5 text-form-label text-subtle"
        data-testid="layout-stage-empty"
      >
        This project has no desktops yet. Open a window to create the first one.
      </div>
    );
  }

  return (
    <div
      className="@container/stage overflow-hidden rounded-lg bg-card shadow-card"
      data-testid="layout-stage"
    >
      <LayoutDesktopTabs
        activeIndex={index}
        desktops={document.desktops}
        onRename={(position, name) => {
          const target = document.desktops[position];
          if (target) editor.edit(renameLayoutDesktop(document, target.id, name));
        }}
        onSelect={setActiveIndex}
      />
      <LayoutTabStrip
        desktop={desktop}
        document={document}
        selectedTabId={tab.id}
        onSelect={selection.selectTab}
      />

      <div className="grid @min-layout-stage-split/stage:grid-cols-[minmax(0,1fr)_var(--width-layout-inspector)]">
        <div className="flex min-w-0 flex-col gap-2 p-4">
          <LayoutCanvas
            document={document}
            selectedWindowId={selection.windowId}
            tab={tab}
            onRootChange={root =>
              editor.edit(replaceLayoutTabRoot(document, desktop.id, tab.id, root))
            }
            onSelect={selection.selectWindow}
          />
        </div>
        <div className="border-t border-line-soft @min-layout-stage-split/stage:border-t-0 @min-layout-stage-split/stage:border-l">
          <LayoutInspector
            desktop={desktop}
            document={document}
            tab={tab}
            windowId={selection.windowId}
            onChange={editor.edit}
            onReveal={selection.reveal}
          />
        </div>
      </div>

      <LayoutReviewBar editor={editor} />
    </div>
  );
}
