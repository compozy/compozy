import { cn } from "@compozy/ui";

import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import type { LayoutProjection, PixelRect } from "../lib/window-manager-types";

function thumbnailStyle(rect: PixelRect, workArea: PixelRect): React.CSSProperties {
  const width = Math.max(1, workArea.w);
  const height = Math.max(1, workArea.h);
  return {
    left: `${((rect.x - workArea.x) / width) * 100}%`,
    top: `${((rect.y - workArea.y) / height) * 100}%`,
    width: `${(rect.w / width) * 100}%`,
    height: `${(rect.h / height) * 100}%`,
  };
}

interface ThumbnailTile {
  key: string;
  rect: PixelRect;
  windowId: string;
  /** Other tabs sharing this frame. */
  others: number;
  floating: boolean;
}

/**
 * Tiles from the authoritative projection: one per stack (its active tab, +N
 * for the rest) and one per unstacked tiled window, then floating windows on
 * top. Minimized windows leave no tile.
 */
function thumbnailTiles(
  projection: LayoutProjection | undefined,
  windows: readonly OsWindow[]
): ThumbnailTile[] {
  const tiled: ThumbnailTile[] = [];
  const floating: ThumbnailTile[] = [];
  for (const window of windows) {
    if (window.minimized) continue;
    const tile = { key: window.id, rect: window.rect, windowId: window.id, others: 0 };
    if (window.placement === "floating") floating.push({ ...tile, floating: true });
    // Without a projection, fall back to each stack's active member.
    else if (!projection && (!window.stackId || window.stackActive)) {
      tiled.push({ ...tile, floating: false });
    }
  }
  if (projection) {
    for (const stack of projection.stacks) {
      tiled.push({
        key: stack.nodeId,
        rect: stack.rect,
        windowId: stack.activeWindowId,
        others: stack.windowIds.length - 1,
        floating: false,
      });
    }
    for (const window of projection.windows) {
      if (window.stackId !== null) continue;
      tiled.push({
        key: window.windowId,
        rect: window.rect,
        windowId: window.windowId,
        others: 0,
        floating: false,
      });
    }
  }
  return [...tiled, ...floating];
}

/**
 * A miniature of one desktop (shell-rail v2 `.mw`): every visible frame as a
 * surface tile with a head strip and its window's title (+N for its other
 * tabs), laid out from the authoritative projection. Presentational only.
 */
export function DesktopLayoutThumbnail({
  projection,
  windows,
}: {
  projection: LayoutProjection | undefined;
  windows: readonly OsWindow[];
}) {
  const workArea = projection?.workArea ?? { x: 0, y: 0, w: 1, h: 1 };
  const apps = new Map(windows.map(window => [window.id, window.app]));
  return (
    <span className="relative block size-full">
      {thumbnailTiles(projection, windows).map(tile => {
        const app = apps.get(tile.windowId);
        const title = app ? getOsAppDescriptor(app).title : "";
        return (
          <span
            key={tile.key}
            data-slot="desktop-thumbnail-window"
            className={cn(
              "absolute min-h-px min-w-px overflow-hidden bg-canvas",
              "before:absolute before:inset-x-0 before:top-0 before:h-2.25 before:border-b before:border-line-soft before:bg-surface-2",
              tile.floating ? "rounded-xs shadow-card" : "ring-[0.5px] ring-line"
            )}
            style={thumbnailStyle(tile.rect, workArea)}
          >
            <span className="absolute inset-x-1.75 top-3.75 truncate text-badge font-medium text-fg-2">
              {tile.others > 0 ? `${title} +${tile.others}` : title}
            </span>
          </span>
        );
      })}
    </span>
  );
}
