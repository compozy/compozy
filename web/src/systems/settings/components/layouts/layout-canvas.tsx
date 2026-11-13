import { Fragment } from "react";

import { cn } from "@compozy/ui";

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
 * One tab's content, laid out the way the shell lays it out
 * (`os-split-view.tsx`): each split is a flex row or column whose members grow
 * by their weights, separated by seams. There are no frames to drag — a tab is
 * its split tree and nothing else.
 */
export function LayoutCanvas({
  document,
  tab,
  selectedWindowId,
  onSelect,
  onRootChange,
}: LayoutCanvasProps) {
  const renderNode = (node: WindowManagerLayoutNode) => {
    if (node.kind === "leaf") {
      return (
        <LayoutCanvasTile
          selected={node.windowId === selectedWindowId}
          window={document.windows[node.windowId]}
          windowId={node.windowId}
          onSelect={() => onSelect(node.windowId)}
        />
      );
    }
    return (
      <div
        className={cn(
          "flex min-h-0 min-w-0 flex-1",
          node.axis === "horizontal" ? "flex-row" : "flex-col"
        )}
        data-split-id={node.id}
        data-testid={`layout-canvas-split-${node.id}`}
      >
        {node.children.map((child, index) => (
          <Fragment key={child.id}>
            {index > 0 ? (
              <LayoutCanvasSeam
                index={index - 1}
                root={tab.root}
                split={node}
                onChange={onRootChange}
              />
            ) : null}
            <div
              className="flex min-h-0 min-w-0"
              style={{ flex: `${node.weights[index] ?? 1} 1 0` }}
            >
              {renderNode(child)}
            </div>
          </Fragment>
        ))}
      </div>
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
      {renderNode(tab.root)}
    </div>
  );
}
