"use client";

import * as React from "react";

import { Button } from "../button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../collapsible";
import { CodeBlock, CopyIconButton } from "./code-block";
import { useResolvedCodeTheme } from "./hooks/use-code-block";
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from "../dialog";
import { OverlayContainerContext } from "../hooks/use-overlay-container";

/** Renders Mermaid in a stable transcript frame while preserving source access and fallback. */
export function MarkdownMermaid({ code, streaming }: { code: string; streaming: boolean }) {
  const theme = useResolvedCodeTheme("auto");
  const [rendered, setRendered] = React.useState<{
    code: string;
    theme: string;
    src: string;
  } | null>(null);
  const src =
    !streaming && rendered?.code === code && rendered.theme === theme ? rendered.src : null;

  React.useEffect(() => {
    if (streaming) return;
    let cancelled = false;
    void import("../../lib/mermaid-renderer")
      .then(({ renderMermaidImage }) => renderMermaidImage(code, theme))
      .then(src => {
        if (!cancelled) setRendered({ code, theme, src });
      })
      .catch(() => {
        // Incomplete/invalid diagrams and unavailable chunks retain the readable source.
        if (!cancelled) setRendered(null);
      });
    return () => {
      cancelled = true;
    };
  }, [code, theme, streaming]);

  const source = (
    <CodeBlock code={code} language="mermaid" caption="mermaid" copyable density="compact" />
  );
  return (
    <div data-slot="markdown-mermaid" className="my-5">
      <div className="flex h-64 max-h-[40vh] items-center overflow-auto overscroll-contain">
        {src ? (
          <img
            src={src}
            alt="Mermaid diagram"
            className="h-auto max-h-full w-auto max-w-1/2 object-contain"
            onError={() => setRendered(null)}
          />
        ) : (
          <div className="w-full self-start">{source}</div>
        )}
      </div>
      <Collapsible>
        <div className="flex items-center gap-2">
          <CollapsibleTrigger disabled={!src} render={<Button variant="ghost" size="sm" />}>
            View source
          </CollapsibleTrigger>
          <CopyIconButton value={code} />
          <MermaidFullscreen src={src} />
        </div>
        <CollapsibleContent>{source}</CollapsibleContent>
      </Collapsible>
    </div>
  );
}

const MIN_ZOOM = 0.25;
const MAX_ZOOM = 4;
const ZOOM_STEP = 0.25;
const PAN_STEP = 24;
// Wheel/pinch zoom is proportional to the pixel delta, capped so one mouse-wheel notch
// stays close to a button step while trackpad pinch deltas scale smoothly.
const WHEEL_ZOOM_RATE = 0.01;
const WHEEL_DELTA_LIMIT = 25;
const WHEEL_LINE_HEIGHT = 16;

type DiagramView = { zoom: number; x: number; y: number };
type ViewerPoint = { x: number; y: number };

const INITIAL_VIEW: DiagramView = { zoom: 1, x: 0, y: 0 };

/**
 * Rescales the view so `anchor`, measured from the viewer centre, stays under the same
 * diagram point. The image is centred in the viewer and scaled around its own centre.
 */
function zoomView(view: DiagramView, zoom: number, anchor: ViewerPoint = { x: 0, y: 0 }) {
  const next = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom));
  const ratio = next / view.zoom;
  return {
    zoom: next,
    x: anchor.x - (anchor.x - view.x) * ratio,
    y: anchor.y - (anchor.y - view.y) * ratio,
  };
}

function zoomModifierLabel() {
  const platform = typeof navigator === "undefined" ? "" : navigator.platform;
  return /^(?:Mac|iPhone|iPad|iPod)/i.test(platform) ? "⌘" : "Ctrl";
}

/** Keeps diagram inspection at viewport scope, independent of the transcript's window portal. */
function MermaidFullscreen({ src }: { src: string | null }) {
  const [view, setView] = React.useState(INITIAL_VIEW);
  // Last pointer position of an active drag.
  const drag = React.useRef<ViewerPoint | null>(null);
  const reset = () => setView(INITIAL_VIEW);
  const zoomBy = (step: number) => setView(value => zoomView(value, value.zoom + step));
  // React wheel listeners are passive; Ctrl/Cmd+wheel and pinch must be cancelled before
  // the browser or Electron applies page zoom. Plain wheel keeps its default behaviour.
  const bindWheelZoom = (viewer: HTMLDivElement) => {
    const onWheel = (event: WheelEvent) => {
      if (!event.ctrlKey && !event.metaKey) return;
      event.preventDefault();
      const bounds = viewer.getBoundingClientRect();
      const anchor = {
        x: event.clientX - bounds.left - bounds.width / 2,
        y: event.clientY - bounds.top - bounds.height / 2,
      };
      const unit =
        event.deltaMode === WheelEvent.DOM_DELTA_PAGE
          ? bounds.height
          : event.deltaMode === WheelEvent.DOM_DELTA_LINE
            ? WHEEL_LINE_HEIGHT
            : 1;
      const pixels = event.deltaY * unit;
      const delta = Math.min(WHEEL_DELTA_LIMIT, Math.max(-WHEEL_DELTA_LIMIT, pixels));
      setView(value => zoomView(value, value.zoom * Math.exp(-delta * WHEEL_ZOOM_RATE), anchor));
    };
    viewer.addEventListener("wheel", onWheel, { passive: false });
    return () => viewer.removeEventListener("wheel", onWheel);
  };
  return (
    <OverlayContainerContext value={null}>
      <Dialog onOpenChange={reset}>
        <DialogTrigger disabled={!src} render={<Button variant="ghost" size="sm" />}>
          Fullscreen
        </DialogTrigger>
        <DialogContent className="h-[calc(100dvh-2rem)] max-w-[calc(100%-2rem)] grid-rows-[auto_auto_minmax(0,1fr)] sm:max-w-[calc(100%-2rem)]">
          <DialogTitle>Mermaid diagram</DialogTitle>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              disabled={view.zoom <= MIN_ZOOM}
              onClick={() => zoomBy(-ZOOM_STEP)}
            >
              Zoom out
            </Button>
            <output aria-label="Zoom">{Math.round(view.zoom * 100)}%</output>
            <Button
              variant="ghost"
              size="sm"
              disabled={view.zoom >= MAX_ZOOM}
              onClick={() => zoomBy(ZOOM_STEP)}
            >
              Zoom in
            </Button>
            <Button variant="ghost" size="sm" onClick={reset}>
              Reset zoom
            </Button>
            <span className="ms-auto text-meta text-subtle">
              {zoomModifierLabel()}+scroll to zoom · drag to pan
            </span>
          </div>
          <div
            ref={bindWheelZoom}
            role="region"
            aria-label="Diagram viewer"
            aria-description="Use arrow keys to pan, plus or minus to zoom, and 0 to reset."
            tabIndex={0}
            className="flex min-h-0 touch-none items-center justify-center overflow-hidden cursor-grab outline-none focus-visible:shadow-focus-inset active:cursor-grabbing"
            onKeyDown={event => {
              let x = 0;
              let y = 0;
              switch (event.key) {
                case "ArrowLeft":
                  x = -PAN_STEP;
                  break;
                case "ArrowRight":
                  x = PAN_STEP;
                  break;
                case "ArrowUp":
                  y = -PAN_STEP;
                  break;
                case "ArrowDown":
                  y = PAN_STEP;
                  break;
                case " ":
                  // Space is held for Space+drag; it must not scroll or activate anything.
                  event.preventDefault();
                  return;
                default: {
                  // Modified keys stay with the browser, app shortcuts, and page zoom.
                  if (event.ctrlKey || event.metaKey || event.altKey) return;
                  if (event.key === "+" || event.key === "=") zoomBy(ZOOM_STEP);
                  else if (event.key === "-") zoomBy(-ZOOM_STEP);
                  else if (event.key === "0") reset();
                  else return;
                  event.preventDefault();
                  return;
                }
              }
              event.preventDefault();
              setView(value => ({ ...value, x: value.x + x, y: value.y + y }));
            }}
            onMouseDown={event => {
              // Middle-button drag pans instead of starting the browser's autoscroll.
              if (event.button === 1) event.preventDefault();
            }}
            onPointerDown={event => {
              if (event.button !== 0 && event.button !== 1) return;
              event.currentTarget.setPointerCapture(event.pointerId);
              drag.current = { x: event.clientX, y: event.clientY };
            }}
            onPointerMove={event => {
              const last = drag.current;
              if (!last) return;
              // Apply each move as a delta so a wheel zoom during the drag keeps its anchor.
              const dx = event.clientX - last.x;
              const dy = event.clientY - last.y;
              drag.current = { x: event.clientX, y: event.clientY };
              setView(value => ({ ...value, x: value.x + dx, y: value.y + dy }));
            }}
            onPointerUp={() => {
              drag.current = null;
            }}
            onPointerCancel={() => {
              drag.current = null;
            }}
            onLostPointerCapture={() => {
              drag.current = null;
            }}
          >
            {src && (
              <img
                src={src}
                alt="Mermaid diagram"
                draggable={false}
                className="h-auto max-h-full w-auto max-w-full select-none object-contain"
                style={{ transform: `translate(${view.x}px, ${view.y}px) scale(${view.zoom})` }}
              />
            )}
          </div>
        </DialogContent>
      </Dialog>
    </OverlayContainerContext>
  );
}
