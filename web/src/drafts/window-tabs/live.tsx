/**
 * The interactive state: tabs activate, close, reorder and split by dragging, window heads move
 * between edges or out to the bar, and seams resize. It applies the same rules as the stories on
 * a local copy of the layout; nothing reaches a daemon.
 */
import { DragGhost } from "../window-tabs-shared/drag-ghost";
import { WINDOWS } from "../window-tabs-shared/fixtures";
import { OsSplitView } from "../window-tabs-shared/os-split-view";
import { OsTabBar } from "../window-tabs-shared/os-tab-bar";
import { WindowShell } from "../window-tabs-shared/shell";
import { useLiveLayout } from "./hooks/use-live-layout";

export function LivePlayground() {
  const {
    shellRef,
    layout,
    drag,
    contentTab,
    activate,
    closeTab,
    closeWindow,
    newTab,
    startDrag,
    focus,
    balanceSeam,
    resizeSeam,
    unsplit,
  } = useLiveLayout();
  const target = drag?.target ?? null;
  const draggedTabId = drag?.source.kind === "tab" ? drag.source.tab.id : undefined;

  return (
    <div className="contents" ref={shellRef}>
      <WindowShell
        bar={
          <OsTabBar
            activeTabId={layout.activeTabId}
            insertionIndex={target?.kind === "bar" ? target.index : null}
            onActivate={activate}
            onClose={closeTab}
            onNewTab={newTab}
            onTabPointerDown={(tabId, event) => {
              const tab = layout.tabs.find(item => item.id === tabId);
              if (tab) startDrag({ kind: "tab", tab }, event);
            }}
            tabProps={draggedTabId ? { [draggedTabId]: { dragging: true } } : undefined}
            tabs={layout.tabs}
            windows={WINDOWS}
          />
        }
        overlay={
          drag ? (
            <DragGhost
              refused={target === null}
              source={drag.source}
              windows={WINDOWS}
              x={drag.x}
              y={drag.y}
            />
          ) : null
        }
      >
        <OsSplitView
          draggingId={drag?.source.kind === "head" ? drag.source.windowId : undefined}
          drop={
            target?.kind === "window" ? { windowId: target.windowId, edge: target.edge } : undefined
          }
          focusedId={layout.focusedId}
          onClose={closeWindow}
          onFocus={focus}
          onHeadPointerDown={(windowId, event) => startDrag({ kind: "head", windowId }, event)}
          onSeamBalance={balanceSeam}
          onSeamPointerDown={resizeSeam}
          onUnsplit={unsplit}
          root={contentTab.root}
          rootDrop={target?.kind === "root" ? target.edge : undefined}
          windows={WINDOWS}
        />
      </WindowShell>
    </div>
  );
}
