import type { RefObject } from "react";

import { cn } from "@agh/ui";

import { clampFloatingRect } from "@/systems/os";

import { usePointerDrag } from "../../hooks/use-pointer-drag";
import { normalizedPercentStyle } from "../../lib/window-manager-layout-canvas";
import { LAYOUT_CANVAS_REFERENCE } from "../../lib/window-manager-layout-reference";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type {
  WindowManagerLayoutWindow,
  WindowManagerNormalizedRect,
} from "../../lib/window-manager-layout-types";

interface LayoutCanvasFloatingProps {
  window: WindowManagerLayoutWindow;
  selected: boolean;
  canvasRef: RefObject<HTMLElement | null>;
  onSelect: () => void;
  onChange: (rect: WindowManagerNormalizedRect) => void;
}

const KEYBOARD_STEP = 0.01;

function toReferencePixels(rect: WindowManagerNormalizedRect) {
  return {
    x: rect.x * LAYOUT_CANVAS_REFERENCE.w,
    y: rect.y * LAYOUT_CANVAS_REFERENCE.h,
    w: rect.w * LAYOUT_CANVAS_REFERENCE.w,
    h: rect.h * LAYOUT_CANVAS_REFERENCE.h,
  };
}

/**
 * A floating window sits above every group on its desktop. Dragging it writes
 * `floatingRect`, clamped by the runtime's own containment rule so it cannot be
 * parked outside the work area the shell will restore it into.
 */
export function LayoutCanvasFloating({
  window,
  selected,
  canvasRef,
  onSelect,
  onChange,
}: LayoutCanvasFloatingProps) {
  const face = layoutWindowFace(window, window.id);
  const Icon = face.icon;

  const move = (rect: WindowManagerNormalizedRect, dx: number, dy: number) => {
    const pixels = toReferencePixels(rect);
    const clamped = clampFloatingRect({
      proposedRect: { ...pixels, x: pixels.x + dx, y: pixels.y + dy },
      workArea: LAYOUT_CANVAS_REFERENCE,
    });
    onChange({
      x: clamped.x / LAYOUT_CANVAS_REFERENCE.w,
      y: clamped.y / LAYOUT_CANVAS_REFERENCE.h,
      w: clamped.w / LAYOUT_CANVAS_REFERENCE.w,
      h: clamped.h / LAYOUT_CANVAS_REFERENCE.h,
    });
  };

  const drag = usePointerDrag<{
    rect: WindowManagerNormalizedRect;
    originX: number;
    originY: number;
    scaleX: number;
    scaleY: number;
  }>({
    onStart: event => {
      const canvas = canvasRef.current?.getBoundingClientRect();
      if (!canvas || canvas.width === 0 || canvas.height === 0) return null;
      onSelect();
      return {
        rect: window.floatingRect,
        originX: event.clientX,
        originY: event.clientY,
        scaleX: LAYOUT_CANVAS_REFERENCE.w / canvas.width,
        scaleY: LAYOUT_CANVAS_REFERENCE.h / canvas.height,
      };
    },
    onMove: (event, state) => {
      move(
        state.rect,
        (event.clientX - state.originX) * state.scaleX,
        (event.clientY - state.originY) * state.scaleY
      );
    },
  });

  return (
    <button
      aria-label={`${face.title}, floating window`}
      aria-pressed={selected}
      className={cn(
        "absolute z-4 flex cursor-grab flex-col overflow-hidden rounded-sm border text-left",
        "border-line-focus bg-elevated shadow-overlay",
        "focus-visible:outline-none focus-visible:shadow-focus-ring active:cursor-grabbing",
        selected && "border-accent",
        window.minimized && "border-dashed opacity-45"
      )}
      data-selected={selected ? "true" : undefined}
      data-testid={`layout-canvas-floating-${window.id}`}
      style={normalizedPercentStyle(window.floatingRect)}
      type="button"
      onClick={event => {
        event.stopPropagation();
        onSelect();
      }}
      onKeyDown={event => {
        const horizontal = event.key === "ArrowLeft" ? -1 : event.key === "ArrowRight" ? 1 : 0;
        const vertical = event.key === "ArrowUp" ? -1 : event.key === "ArrowDown" ? 1 : 0;
        if (horizontal === 0 && vertical === 0) return;
        event.preventDefault();
        event.stopPropagation();
        const step = KEYBOARD_STEP * (event.shiftKey ? 10 : 1);
        move(
          window.floatingRect,
          horizontal * step * LAYOUT_CANVAS_REFERENCE.w,
          vertical * step * LAYOUT_CANVAS_REFERENCE.h
        );
      }}
      onPointerDown={drag.onPointerDown}
    >
      <span className="flex h-4 flex-none items-center gap-1 border-b border-line-soft px-1.5">
        <Icon aria-hidden="true" className="size-2.5 shrink-0 text-subtle" />
        <span className="truncate text-workspace-avatar font-semibold text-fg">{face.title}</span>
      </span>
      <span className="flex min-h-0 flex-1 items-center justify-center px-1">
        <span className="truncate font-mono text-workspace-avatar text-subtle">{face.detail}</span>
      </span>
    </button>
  );
}
