import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
} from "./window-manager-layout-types";

export interface LayoutThumbnailTile {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface LayoutThumbnail {
  viewBox: string;
  tiles: LayoutThumbnailTile[];
  /** Tabs on the pictured desktop; the picture shows the one it opens on. */
  tabCount: number;
}

const VIEW_WIDTH = 100;
const VIEW_HEIGHT = 62;
const SEAM = 1.6;
const INSET = 1;

/**
 * A card's picture of a saved layout: the first desktop's default tab, its
 * split drawn from the weights. Deliberately not a projection — no gaps, no
 * window minimums, no viewport — a stored document has no screen to be
 * measured against. This is a diagram of the shape.
 */
export function layoutThumbnail(document: WindowManagerLayoutDocument): LayoutThumbnail {
  const desktop = document.desktops[0];
  const tiles: LayoutThumbnailTile[] = [];
  const tab =
    desktop?.tabs.find(candidate => candidate.id === desktop.activeTabId) ?? desktop?.tabs[0];
  if (tab) {
    collect(
      tab.root,
      { x: INSET, y: INSET, w: VIEW_WIDTH - INSET * 2, h: VIEW_HEIGHT - INSET * 2 },
      tiles
    );
  }
  return {
    viewBox: `0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`,
    tiles,
    tabCount: desktop?.tabs.length ?? 0,
  };
}

function collect(
  node: WindowManagerLayoutNode,
  rect: LayoutThumbnailTile,
  tiles: LayoutThumbnailTile[]
): void {
  if (node.kind === "leaf") {
    tiles.push(rect);
    return;
  }
  const horizontal = node.axis === "horizontal";
  const seams = (node.children.length - 1) * SEAM;
  const span = Math.max(0, (horizontal ? rect.w : rect.h) - seams);
  let offset = horizontal ? rect.x : rect.y;
  node.children.forEach((child, index) => {
    const length = span * (node.weights[index] ?? 0);
    collect(
      child,
      horizontal
        ? { x: offset, y: rect.y, w: length, h: rect.h }
        : { x: rect.x, y: offset, w: rect.w, h: length },
      tiles
    );
    offset += length + SEAM;
  });
}
