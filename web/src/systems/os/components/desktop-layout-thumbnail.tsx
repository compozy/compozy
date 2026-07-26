import { cn } from "@agh/ui";

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

export function DesktopLayoutThumbnail({
  projection,
  windows,
}: {
  projection: LayoutProjection | undefined;
  windows: readonly OsWindow[];
}) {
  const workArea = projection?.workArea ?? { x: 0, y: 0, w: 1, h: 1 };
  return (
    <span className="relative block size-full bg-canvas-tint">
      {windows.map(window =>
        window.minimized ? null : (
          <span
            key={window.id}
            className={cn(
              "absolute min-h-px min-w-px rounded-xs border border-line-strong bg-elevated",
              window.placement === "floating" && "shadow-elevated"
            )}
            style={thumbnailStyle(window.rect, workArea)}
          />
        )
      )}
    </span>
  );
}
