/**
 * The canvas renders the selected prototype in an iframe on its standalone URL
 * (`/<slug>?chrome=0&theme=…&state=…`), so themes and global styles stay isolated and the
 * screenshot URL is exactly what reviewers see. Widths wider than the area scale down to fit.
 * Both edges of the frame drag (or take arrow keys) to any width from 320 to 3840 px, kept in
 * `?w=` on release. `children` render over the reviewed frame (the review overlay slot).
 */
import { useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from "react";

import { cn } from "@compozy/ui";

import { Frame } from "#gallery/shell/frame";
import type { FrameInfo } from "#gallery/shell/use-frame";
import { useElementSize } from "#gallery/shell/use-element-size";
import {
  frameSrc,
  maxWidth,
  minWidth,
  updateView,
  type Theme,
  type Viewport,
} from "#gallery/shell/view";

type CanvasProps = {
  slug: string;
  title: string;
  state: string;
  theme: Theme | null;
  width: Viewport;
  compare: boolean;
  onInfo: (info: FrameInfo | null) => void;
  children?: ReactNode;
};

type Fitted = { width: number; height: number; scale: number };

function fit(available: { width: number; height: number }, viewport: Viewport): Fitted {
  const width = viewport === "fit" ? available.width : viewport;
  const scale = width > 0 ? Math.min(1, available.width / width) : 1;
  return { width, height: Math.floor(available.height / scale), scale };
}

const clampWidth = (width: number) => Math.round(Math.min(maxWidth, Math.max(minWidth, width)));

const comparedThemes: readonly Theme[] = ["light", "dark"];

type Drag = { side: -1 | 1; x: number; width: number; scale: number };

type EdgeProps = {
  side: -1 | 1;
  width: number;
  scale: number;
  onDrag: (width: number | null) => void;
};

/**
 * A draggable edge of the frame. The frame stays centered, so moving one edge by `d` screen px
 * changes the width by `2d` (divided by the scale, for frames wider than the area).
 */
function Edge({ side, width, scale, onDrag }: EdgeProps) {
  const drag = useRef<Drag | null>(null);
  const widthAt = (event: PointerEvent) => {
    const start = drag.current;
    if (!start) return width;
    return clampWidth(start.width + (2 * start.side * (event.clientX - start.x)) / start.scale);
  };
  const onKeyDown = (event: KeyboardEvent) => {
    const delta = { ArrowLeft: -side, ArrowRight: side }[event.key];
    if (delta === undefined) return;
    event.preventDefault();
    updateView({ width: clampWidth(width + delta * (event.shiftKey ? 100 : 10)) });
  };
  const onPointerUp = (event: PointerEvent) => {
    const next = widthAt(event);
    drag.current = null;
    onDrag(null);
    updateView({ width: next });
  };
  return (
    <button
      aria-label={side === 1 ? "Resize from the right edge" : "Resize from the left edge"}
      className={cn(
        "group/edge absolute top-0 z-10 flex h-full w-3 cursor-ew-resize touch-none items-center justify-center rounded-md outline-none focus-visible:shadow-focus-ring",
        side === 1 ? "-right-3" : "-left-3"
      )}
      data-testid={side === 1 ? "canvas-edge-right" : "canvas-edge-left"}
      onKeyDown={onKeyDown}
      onPointerCancel={() => {
        drag.current = null;
        onDrag(null);
      }}
      onPointerDown={event => {
        event.currentTarget.setPointerCapture(event.pointerId);
        drag.current = { side, x: event.clientX, width, scale };
        onDrag(width);
      }}
      onPointerMove={event => {
        if (drag.current) onDrag(widthAt(event));
      }}
      onPointerUp={onPointerUp}
      title={`${width} px`}
      type="button"
    >
      <span className="h-10 w-1 rounded-pill bg-line-strong transition-colors group-hover/edge:bg-fg-3 group-active/edge:bg-accent" />
    </button>
  );
}

/** Light and dark side by side, in the same state and width. */
function CompareFrames({
  props,
  frame,
  height,
}: {
  props: CanvasProps;
  frame: Fitted;
  height: number;
}) {
  return comparedThemes.map(theme => (
    <div className="flex min-w-0 flex-1 flex-col gap-1.5" key={theme}>
      <span className="font-mono text-mono-id tracking-mono-id text-subtle">{theme}</span>
      <div
        className="overflow-hidden rounded-lg bg-canvas shadow-hairline"
        style={{ width: frame.width * frame.scale, height: height - 22 }}
      >
        <Frame
          height={Math.floor((height - 22) / frame.scale)}
          scale={frame.scale}
          src={frameSrc(props.slug, { state: props.state, theme })}
          title={`${props.title} · ${theme}`}
          width={frame.width}
        />
      </div>
    </div>
  ));
}

/** The reviewed frame: overlay slot, drag edges and its width (and scale) below. */
function SingleFrame({
  props,
  frame,
  height,
  dragWidth,
  onDrag,
}: {
  props: CanvasProps;
  frame: Fitted;
  height: number;
  dragWidth: number | null;
  onDrag: (width: number | null) => void;
}) {
  return (
    <div className="relative" style={{ width: frame.width * frame.scale, height }}>
      <div className="relative size-full overflow-hidden rounded-lg bg-canvas shadow-hairline">
        <Frame
          height={frame.height}
          onInfo={props.onInfo}
          scale={frame.scale}
          src={frameSrc(props.slug, { state: props.state, theme: props.theme })}
          title={props.title}
          width={frame.width}
        />
        {props.children}
        {/* While dragging, the iframe must not swallow the pointer. */}
        {dragWidth === null ? null : <div className="absolute inset-0 cursor-ew-resize" />}
      </div>
      {([-1, 1] as const).map(side => (
        <Edge
          key={side}
          onDrag={onDrag}
          scale={frame.scale}
          side={side}
          width={Math.round(frame.width)}
        />
      ))}
      <span
        aria-live="polite"
        className={cn(
          "pointer-events-none absolute top-full left-1/2 mt-1 -translate-x-1/2 font-mono text-mono-id tracking-mono-id whitespace-nowrap text-subtle",
          dragWidth !== null && "text-fg"
        )}
        data-testid="canvas-size"
      >
        {Math.round(frame.width)} px
        {frame.scale < 1 ? ` · ${Math.round(frame.scale * 100)}%` : ""}
      </span>
    </div>
  );
}

export function Canvas(props: CanvasProps) {
  const [ref, size] = useElementSize<HTMLDivElement>();
  const [dragWidth, setDragWidth] = useState<number | null>(null);
  const columns = props.compare ? 2 : 1;
  // Room between compared frames, for the drag edges of a single frame and for its size below.
  const reserved = props.compare ? { width: 12, height: 0 } : { width: 32, height: 20 };
  const area = {
    width: Math.max(0, (size.width - reserved.width) / columns),
    height: Math.max(0, size.height - reserved.height),
  };
  const frame = fit(area, dragWidth ?? props.width);
  const ready = size.width > 0 && size.height > 0;

  return (
    <div
      className="relative flex min-h-0 min-w-0 flex-1 justify-center gap-3 rounded-xl bg-surface-2 p-2"
      data-testid="canvas"
      ref={ref}
    >
      {!ready ? null : props.compare ? (
        <CompareFrames frame={frame} height={size.height} props={props} />
      ) : (
        <SingleFrame
          dragWidth={dragWidth}
          frame={frame}
          height={area.height}
          onDrag={setDragWidth}
          props={props}
        />
      )}
    </div>
  );
}
