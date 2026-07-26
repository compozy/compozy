import { cn } from "@agh/ui";

import type { ProjectedStack, ProjectedWindow } from "@/systems/os";

import { canvasPercentStyle } from "../../lib/window-manager-layout-canvas";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type { WindowManagerLayoutDocument } from "../../lib/window-manager-layout-types";

interface LayoutCanvasTileProps {
  document: WindowManagerLayoutDocument;
  projected: ProjectedWindow;
  stack: ProjectedStack | null;
  selected: boolean;
  onSelect: () => void;
}

/**
 * One projected window. Purely presentational: the rect comes from the runtime
 * projector, so what shows here is the arrangement the shell will actually build.
 */
export function LayoutCanvasTile({
  document,
  projected,
  stack,
  selected,
  onSelect,
}: LayoutCanvasTileProps) {
  const face = layoutWindowFace(document.windows[projected.windowId], projected.windowId);
  const Icon = face.icon;

  return (
    <button
      aria-pressed={selected}
      className={cn(
        "absolute flex min-h-0 min-w-0 flex-col overflow-hidden rounded-sm border text-left",
        "border-line-strong bg-surface-glaze transition-colors duration-base ease-out",
        "hover:border-line-focus focus-visible:outline-none focus-visible:shadow-focus-ring",
        selected && "border-accent bg-accent-tint"
      )}
      data-selected={selected ? "true" : undefined}
      data-testid={`layout-canvas-tile-${projected.windowId}`}
      style={canvasPercentStyle(projected.rect)}
      type="button"
      onClick={event => {
        event.stopPropagation();
        onSelect();
      }}
    >
      {stack ? <LayoutStackTabs document={document} stack={stack} /> : null}
      <span className="flex h-4 flex-none items-center gap-1 border-b border-line-soft bg-canvas px-1.5">
        <Icon aria-hidden="true" className="size-2.5 shrink-0 text-subtle" />
        <span className="truncate text-workspace-avatar font-semibold text-fg">{face.title}</span>
      </span>
      <span className="flex min-h-0 flex-1 items-center justify-center px-1">
        <span className="truncate font-mono text-workspace-avatar text-subtle">{face.detail}</span>
      </span>
      {projected.adapted ? (
        <span className="absolute right-1 bottom-1 rounded-xxs bg-warning-tint px-1 text-workspace-avatar font-semibold text-warning">
          Folded
        </span>
      ) : null}
    </button>
  );
}

function LayoutStackTabs({
  document,
  stack,
}: {
  document: WindowManagerLayoutDocument;
  stack: ProjectedStack;
}) {
  return (
    <span className="flex flex-none gap-0.5 px-1 pt-1">
      {stack.windowIds.map(windowId => {
        const face = layoutWindowFace(document.windows[windowId], windowId);
        return (
          <span
            className={cn(
              "min-w-0 flex-1 truncate rounded-t-xxs border border-b-0 border-line-soft bg-canvas px-1",
              "text-workspace-avatar text-subtle",
              windowId === stack.activeWindowId && "border-line-strong bg-elevated text-fg"
            )}
            key={windowId}
          >
            {face.title}
          </span>
        );
      })}
    </span>
  );
}
