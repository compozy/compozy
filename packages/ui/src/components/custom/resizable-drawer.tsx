"use client";

import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import { useRef, useState, type ReactNode } from "react";

import { Button } from "../button";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "../sheet";
import { useInlineLayout } from "./hooks/use-inline-layout";

export interface ResizableDrawerProps {
  children: ReactNode;
  content: ReactNode;
  title: ReactNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  width: number;
  onWidthChange: (width: number) => void;
  minWidth?: number;
  maxWidth?: number;
  inlineBreakpoint?: number;
  returnFocus?: HTMLElement | null;
}

/** A focus-contained detail pane; pointer access to the host can select another item. */
export function ResizableDrawer({
  children,
  content,
  title,
  open,
  onOpenChange,
  width,
  onWidthChange,
  minWidth = 360,
  maxWidth = 800,
  inlineBreakpoint = 1440,
  returnFocus,
}: ResizableDrawerProps) {
  const inline = useInlineLayout(inlineBreakpoint);
  const [container, setContainer] = useState<HTMLDivElement | null>(null);
  const drag = useRef<{ x: number; width: number } | null>(null);
  const finalFocus = () => (returnFocus?.isConnected ? returnFocus : true);
  const resize = (next: number) => {
    const maximum = Math.min(maxWidth, (container?.clientWidth ?? maxWidth / 0.7) * 0.7);
    onWidthChange(Math.max(Math.min(minWidth, maximum), Math.min(maximum, next)));
  };
  return (
    <DialogPrimitive.Root
      open={open && inline}
      onOpenChange={onOpenChange}
      modal="trap-focus"
      disablePointerDismissal
    >
      <div ref={setContainer} className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
        <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">{children}</div>
        {open && inline && container && (
          <DialogPrimitive.Portal container={container} className="contents">
            <DialogPrimitive.Popup
              data-slot="resizable-drawer"
              data-mode="inline"
              finalFocus={finalFocus}
              className="relative flex min-h-0 min-w-0 shrink-0 flex-col border-l border-line bg-canvas outline-none"
              style={{ width: Math.max(minWidth, Math.min(maxWidth, width)), maxWidth: "70%" }}
            >
              {/* PanelGroup requires direct DOM siblings; this separator must live inside the dialog's focus boundary. */}
              <div
                role="separator"
                aria-label="Resize drawer"
                aria-orientation="vertical"
                aria-valuemin={minWidth}
                aria-valuemax={maxWidth}
                aria-valuenow={Math.round(width)}
                tabIndex={0}
                className="absolute inset-y-0 -left-0.5 z-10 w-1 cursor-col-resize touch-none hover:bg-line-strong focus-visible:bg-line-strong focus-visible:outline-none"
                onPointerDown={event => {
                  if (event.button !== 0) return;
                  event.preventDefault();
                  event.currentTarget.focus();
                  event.currentTarget.setPointerCapture(event.pointerId);
                  drag.current = {
                    x: event.clientX,
                    width:
                      event.currentTarget.parentElement?.getBoundingClientRect().width ?? width,
                  };
                }}
                onPointerMove={event => {
                  if (drag.current) resize(drag.current.width + drag.current.x - event.clientX);
                }}
                onPointerUp={event => {
                  drag.current = null;
                  event.currentTarget.releasePointerCapture(event.pointerId);
                }}
                onLostPointerCapture={() => {
                  drag.current = null;
                }}
                onKeyDown={event => {
                  const next =
                    event.key === "ArrowLeft"
                      ? width + 16
                      : event.key === "ArrowRight"
                        ? width - 16
                        : event.key === "Home"
                          ? minWidth
                          : event.key === "End"
                            ? maxWidth
                            : null;
                  if (next === null) return;
                  event.preventDefault();
                  event.stopPropagation();
                  resize(next);
                }}
              />
              <header className="flex shrink-0 items-start gap-2 border-b border-line p-4">
                <DialogPrimitive.Title className="min-w-0 flex-1 text-item-title font-medium text-fg-strong">
                  {title}
                </DialogPrimitive.Title>
                <DialogPrimitive.Close
                  render={<Button variant="quiet" size="icon-sm" aria-label="Close drawer" />}
                >
                  <XIcon />
                </DialogPrimitive.Close>
              </header>
              <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{content}</div>
            </DialogPrimitive.Popup>
          </DialogPrimitive.Portal>
        )}
      </div>
      <Sheet open={open && !inline} onOpenChange={onOpenChange} modal="trap-focus">
        <SheetContent
          data-slot="resizable-drawer"
          data-mode="overlay"
          finalFocus={finalFocus}
          className="w-full gap-0 sm:max-w-none"
          style={{ width: `min(${width}px, 100%)` }}
        >
          <SheetHeader className="shrink-0 border-b border-line pr-12">
            <SheetTitle>{title}</SheetTitle>
          </SheetHeader>
          <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{content}</div>
        </SheetContent>
      </Sheet>
    </DialogPrimitive.Root>
  );
}
