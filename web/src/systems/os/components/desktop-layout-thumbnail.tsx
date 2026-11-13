import { cn } from "@compozy/ui";

import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import type { PixelRect, TabProjection } from "../lib/window-manager-types";

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
  /** Other members a collapsed split keeps behind its switcher. */
  others: number;
}

/**
 * Tiles from the active tab's projection: one per window laid out side by side, and one per
 * collapsed split (its shown member, +N for the rest).
 */
function thumbnailTiles(projection: TabProjection | undefined): ThumbnailTile[] {
  if (projection === undefined) return [];
  const collapsedWindowIds = new Set(projection.collapsed.flatMap(split => split.windowIds));
  const tiles: ThumbnailTile[] = projection.collapsed.map(split => ({
    key: split.splitId,
    rect: split.rect,
    windowId: split.shownWindowId,
    others: split.windowIds.length - 1,
  }));
  for (const window of projection.windows) {
    if (collapsedWindowIds.has(window.windowId)) continue;
    tiles.push({ key: window.windowId, rect: window.rect, windowId: window.windowId, others: 0 });
  }
  return tiles;
}

/**
 * A miniature of one desktop (shell-rail v2 `.mw`): the active tab's windows as surface tiles
 * with a head strip and the window's title, laid out from the tab's split. Presentational only.
 */
export function DesktopLayoutThumbnail({
  projection,
  windows,
}: {
  projection: TabProjection | undefined;
  windows: readonly OsWindow[];
}) {
  const workArea = projection?.workArea ?? { x: 0, y: 0, w: 1, h: 1 };
  const apps = new Map(windows.map(window => [window.id, window.app]));
  return (
    <span className="relative block size-full">
      {thumbnailTiles(projection).map(tile => {
        const app = apps.get(tile.windowId);
        const title = app ? getOsAppDescriptor(app).title : "";
        return (
          <span
            key={tile.key}
            data-slot="desktop-thumbnail-window"
            className={cn(
              "absolute min-h-px min-w-px overflow-hidden bg-card ring-[0.5px] ring-line",
              "before:absolute before:inset-x-0 before:top-0 before:h-2.25 before:border-b before:border-line-soft before:bg-surface-2"
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
