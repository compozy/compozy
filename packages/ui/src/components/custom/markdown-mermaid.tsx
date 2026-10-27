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

/** Keeps diagram inspection at viewport scope, independent of the transcript's window portal. */
function MermaidFullscreen({ src }: { src: string | null }) {
  const [zoom, setZoom] = React.useState(1);
  const [position, setPosition] = React.useState({ x: 0, y: 0 });
  const drag = React.useRef<{ x: number; y: number } | null>(null);
  const reset = () => {
    setZoom(1);
    setPosition({ x: 0, y: 0 });
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
              disabled={zoom <= 0.25}
              onClick={() => setZoom(value => Math.max(0.25, value - 0.25))}
            >
              Zoom out
            </Button>
            <output aria-label="Zoom">{Math.round(zoom * 100)}%</output>
            <Button
              variant="ghost"
              size="sm"
              disabled={zoom >= 4}
              onClick={() => setZoom(value => Math.min(4, value + 0.25))}
            >
              Zoom in
            </Button>
            <Button variant="ghost" size="sm" onClick={reset}>
              Reset zoom
            </Button>
          </div>
          <div
            role="region"
            aria-label="Diagram viewer"
            aria-description="Use arrow keys to pan."
            tabIndex={0}
            className="flex min-h-0 touch-none items-center justify-center overflow-hidden cursor-grab outline-none focus-visible:shadow-focus-inset active:cursor-grabbing"
            onKeyDown={event => {
              let x = 0;
              let y = 0;
              switch (event.key) {
                case "ArrowLeft":
                  x = -24;
                  break;
                case "ArrowRight":
                  x = 24;
                  break;
                case "ArrowUp":
                  y = -24;
                  break;
                case "ArrowDown":
                  y = 24;
                  break;
                default:
                  return;
              }
              event.preventDefault();
              setPosition(value => ({ x: value.x + x, y: value.y + y }));
            }}
            onPointerDown={event => {
              if (event.button !== 0) return;
              event.currentTarget.setPointerCapture(event.pointerId);
              drag.current = { x: event.clientX - position.x, y: event.clientY - position.y };
            }}
            onPointerMove={event => {
              if (drag.current)
                setPosition({
                  x: event.clientX - drag.current.x,
                  y: event.clientY - drag.current.y,
                });
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
                style={{ transform: `translate(${position.x}px, ${position.y}px) scale(${zoom})` }}
              />
            )}
          </div>
        </DialogContent>
      </Dialog>
    </OverlayContainerContext>
  );
}
