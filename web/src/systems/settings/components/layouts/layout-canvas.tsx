import { cn } from "@compozy/ui";

import { OsSplitView, type OsSplitSeamSlot } from "@/systems/os";

import { findLayoutNode } from "../../lib/window-manager-layout-tree";
import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
  WindowManagerLayoutTab,
} from "../../lib/window-manager-layout-types";
import { LayoutCanvasSeam } from "./layout-canvas-seam";
import { LayoutCanvasTile } from "./layout-canvas-tile";

interface LayoutCanvasProps {
  document: WindowManagerLayoutDocument;
  tab: WindowManagerLayoutTab;
  /** The window whose leaf the inspector is showing. */
  selectedWindowId: string | null;
  onSelect: (windowId: string | null) => void;
  onRootChange: (root: WindowManagerLayoutNode) => void;
}

/**
 * The draft lays nothing out on screen, so no split is ever collapsed and no
 * live window or session backs a tile.
 */
const NO_COLLAPSED: [] = [];
const NO_LIVE_WINDOWS = {};
const noSession = () => undefined;
const noShowMember = () => undefined;

/**
 * One tab's content, laid out by the shell's own split renderer
 * (`OsSplitView`): each split is a flex row or column whose members grow by
 * their weights, separated by seams. The editor supplies the tiles and the
 * seams, which edit the draft instead of the running workspace.
 */
export function LayoutCanvas({
  document,
  tab,
  selectedWindowId,
  onSelect,
  onRootChange,
}: LayoutCanvasProps) {
  const renderSeam = (slot: OsSplitSeamSlot) => {
    const split = findLayoutNode(tab.root, slot.splitId);
    if (split === null || split.kind !== "split") return null;
    return (
      <LayoutCanvasSeam
        index={slot.boundaryIndex}
        root={tab.root}
        split={split}
        onChange={onRootChange}
      />
    );
  };

  return (
    <div
      className={cn(
        "relative flex aspect-[16/10] overflow-hidden rounded-md border border-line-strong bg-canvas",
        "shadow-hairline-inset select-none"
      )}
      data-testid="layout-canvas"
      data-tab-id={tab.id}
    >
      <OsSplitView
        collapsed={NO_COLLAPSED}
        renderSeam={renderSeam}
        renderWindow={windowId => (
          <LayoutCanvasTile
            selected={windowId === selectedWindowId}
            window={document.windows[windowId]}
            windowId={windowId}
            onSelect={() => onSelect(windowId)}
          />
        )}
        root={tab.root}
        sessionOf={noSession}
        windows={NO_LIVE_WINDOWS}
        onShowMember={noShowMember}
      />
    </div>
  );
}
