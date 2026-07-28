import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
} from "./window-manager-layout-types";

export interface LayoutThumbnailTile {
  x: number;
  y: number;
  w: number;
  h: number;
  stacked: boolean;
}

export interface LayoutThumbnail {
  viewBox: string;
  tiles: LayoutThumbnailTile[];
}

const VIEW_WIDTH = 100;
const VIEW_HEIGHT = 62;
const SEAM = 1.6;
const INSET = 1;

/**
 * A card's picture of a saved layout. Deliberately not a projection: no gaps, no
 * window minimums, no viewport — a stored document has no screen to be measured
 * against, and pretending otherwise would show fit warnings for a size nobody
 * chose. This is a diagram of the shape.
 */
export function layoutThumbnail(document: WindowManagerLayoutDocument): LayoutThumbnail {
  const desktop = document.desktops[0];
  const tiles: LayoutThumbnailTile[] = [];
  if (desktop) {
    for (const group of desktop.groups) {
      collect(
        group.root,
        {
          x: INSET + group.frame.x * (VIEW_WIDTH - INSET * 2),
          y: INSET + group.frame.y * (VIEW_HEIGHT - INSET * 2),
          w: group.frame.w * (VIEW_WIDTH - INSET * 2),
          h: group.frame.h * (VIEW_HEIGHT - INSET * 2),
          stacked: false,
        },
        tiles
      );
    }
  }
  return { viewBox: `0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`, tiles };
}

function collect(
  node: WindowManagerLayoutNode,
  rect: LayoutThumbnailTile,
  tiles: LayoutThumbnailTile[]
): void {
  if (node.kind !== "split") {
    tiles.push({ ...rect, stacked: node.kind === "stack" });
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
        ? { x: offset, y: rect.y, w: length, h: rect.h, stacked: false }
        : { x: rect.x, y: offset, w: rect.w, h: length, stacked: false },
      tiles
    );
    offset += length + SEAM;
  });
}
