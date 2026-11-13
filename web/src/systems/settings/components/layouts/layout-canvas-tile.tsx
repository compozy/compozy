import { cn } from "@compozy/ui";

import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type { WindowManagerLayoutWindow } from "../../lib/window-manager-layout-types";

interface LayoutCanvasTileProps {
  window: WindowManagerLayoutWindow | undefined;
  windowId: string;
  selected: boolean;
  onSelect: () => void;
}

/** One window of the tab: its app and route, and the button that opens it in the inspector. */
export function LayoutCanvasTile({ window, windowId, selected, onSelect }: LayoutCanvasTileProps) {
  const face = layoutWindowFace(window, windowId);
  const Icon = face.icon;

  return (
    <button
      aria-pressed={selected}
      className={cn(
        "flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden text-left",
        "bg-surface-2 transition-colors duration-base ease-out",
        "hover:bg-selected focus-visible:outline-none focus-visible:shadow-focus-ring",
        selected && "bg-selected ring-1 ring-fg ring-inset"
      )}
      data-selected={selected ? "true" : undefined}
      data-testid={`layout-canvas-tile-${windowId}`}
      type="button"
      onClick={onSelect}
    >
      <span className="flex h-5 flex-none items-center gap-1 border-b border-line-soft bg-canvas px-1.5">
        <Icon aria-hidden="true" className="size-3 shrink-0 text-subtle" />
        <span className="truncate text-badge font-semibold text-fg">{face.title}</span>
      </span>
      <span className="flex min-h-0 flex-1 items-center justify-center px-1">
        <span className="truncate font-mono text-badge text-subtle">{face.detail}</span>
      </span>
    </button>
  );
}
