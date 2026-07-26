import { useRef } from "react";

import { cn } from "@agh/ui";

import type { LayoutProjection, WindowManagerConfig } from "@/systems/os";

import type { LayoutSelection } from "../../hooks/use-layout-canvas-selection";
import {
  normalizedPercentStyle,
  overlappingGroupIds,
} from "../../lib/window-manager-layout-canvas";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerNormalizedRect,
} from "../../lib/window-manager-layout-types";
import { LayoutCanvasFloating } from "./layout-canvas-floating";
import { LayoutCanvasGroupEdge } from "./layout-canvas-group-edge";
import { LayoutCanvasSeam } from "./layout-canvas-seam";
import { LayoutCanvasTile } from "./layout-canvas-tile";

const GROUP_EDGES = ["top", "right", "bottom", "left"] as const;

interface LayoutCanvasProps {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  projection: LayoutProjection;
  config: WindowManagerConfig;
  selection: LayoutSelection;
  onSelect: (next: LayoutSelection) => void;
  onDesktopChange: (next: WindowManagerLayoutDesktop) => void;
  onFloatingRectChange: (windowId: string, rect: WindowManagerNormalizedRect) => void;
}

/**
 * The desktop, at the size of a desktop. Every rect on this surface comes from
 * the runtime projector, so the arrangement shown is the arrangement the shell
 * builds — including where it folds a split into a stack because the windows no
 * longer fit.
 */
export function LayoutCanvas({
  document,
  desktop,
  projection,
  config,
  selection,
  onSelect,
  onDesktopChange,
  onFloatingRectChange,
}: LayoutCanvasProps) {
  const canvasRef = useRef<HTMLDivElement>(null);
  const overlapping = overlappingGroupIds(desktop);
  const stacks = new Map(projection.stacks.map(stack => [stack.nodeId, stack]));
  const empty = desktop.groups.length === 0 && desktop.floating.length === 0;

  return (
    <div
      className={cn(
        "relative aspect-[16/10] overflow-hidden rounded-md border border-line-strong bg-canvas",
        "shadow-hairline-inset select-none"
      )}
      data-testid="layout-canvas"
      ref={canvasRef}
      onClick={() => onSelect(null)}
    >
      {empty ? (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-1 text-small-body text-subtle">
          <p className="text-fg">No windows on this desktop</p>
          <p>Open a window in the workspace to start tiling.</p>
        </div>
      ) : null}

      {desktop.groups.map(group => (
        <div
          className={cn(
            "absolute rounded-sm outline-1 outline-transparent",
            "transition-[outline-color] duration-base ease-out",
            selection?.kind === "group" &&
              selection.id === group.id &&
              "outline-accent-dim outline-offset-1",
            overlapping.has(group.id) && "outline-dashed outline-danger"
          )}
          data-overlapping={overlapping.has(group.id) ? "true" : undefined}
          data-testid={`layout-canvas-group-${group.id}`}
          key={group.id}
          style={normalizedPercentStyle(group.frame)}
        >
          {GROUP_EDGES.map(edge => (
            <LayoutCanvasGroupEdge
              canvasRef={canvasRef}
              edge={edge}
              group={group}
              key={edge}
              onChange={frame => {
                onSelect({ kind: "group", id: group.id });
                onDesktopChange({
                  ...desktop,
                  groups: desktop.groups.map(candidate =>
                    candidate.id === group.id ? { ...candidate, frame } : candidate
                  ),
                });
              }}
            />
          ))}
        </div>
      ))}

      {projection.windows.map(projected => (
        <LayoutCanvasTile
          document={document}
          key={projected.windowId}
          projected={projected}
          selected={selection?.kind === "node" && selection.id === projected.nodeId}
          stack={
            projected.stackId !== null && projected.active
              ? (stacks.get(projected.stackId) ?? null)
              : null
          }
          onSelect={() => onSelect({ kind: "node", id: projected.nodeId })}
        />
      ))}

      {projection.seams.map(seam => (
        <LayoutCanvasSeam
          canvasRef={canvasRef}
          desktop={desktop}
          key={seam.id}
          seam={seam}
          onChange={onDesktopChange}
        />
      ))}

      {desktop.floating.map(windowId => {
        const window = document.windows[windowId];
        if (!window) return null;
        return (
          <LayoutCanvasFloating
            canvasRef={canvasRef}
            key={windowId}
            selected={selection?.kind === "window" && selection.id === windowId}
            window={window}
            onChange={rect => onFloatingRectChange(windowId, rect)}
            onSelect={() => onSelect({ kind: "window", id: windowId })}
          />
        );
      })}

      <span className="sr-only">
        Gaps of {config.gaps.inner} pixels between tiles, shown at the reference screen size.
      </span>
    </div>
  );
}
