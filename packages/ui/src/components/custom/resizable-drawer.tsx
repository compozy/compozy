"use client";

import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { useRef, useState, type ReactNode } from "react";

import { XIcon } from "../../icons";
import { defaultDialogInitialFocus } from "../../lib/dialog-initial-focus";
import { Button } from "../button";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "../sheet";
import { useContainerInlineLayout, useInlineLayout } from "./hooks/use-inline-layout";

type DrawerInitialFocus = DialogPrimitive.Popup.Props["initialFocus"];
type DrawerOpenChangeDetails = Parameters<
  NonNullable<DialogPrimitive.Root.Props["onOpenChange"]>
>[1];

export interface ResizableDrawerProps {
  children: ReactNode;
  content: ReactNode;
  title: ReactNode;
  actions?: ReactNode;
  open: boolean;
  onOpenChange: (open: boolean, details: DrawerOpenChangeDetails) => void;
  width: number;
  onWidthChange: (width: number) => void;
  minWidth?: number;
  maxWidth?: number;
  inlineBreakpoint?: number;
  /**
   * What `inlineBreakpoint` measures: the viewport (default), or the drawer's own host, so a
   * pane inside a window follows that window. Below it the host is narrow: the drawer then
   * covers the host edge to edge instead of opening at `width`.
   */
  breakpointBasis?: "viewport" | "container";
  /**
   * Replaces the default head (title, actions and the close button). `title` still names the
   * dialog for assistive technology; the replacement owns its own close control.
   */
  header?: ReactNode;
  /** Where focus lands on close; a function resolves it at close time, after the host re-rendered. */
  returnFocus?: HTMLElement | null | (() => HTMLElement | null);
  initialFocus?: DrawerInitialFocus;
  /**
   * The inline pane's modality. `trap-focus` (default) contains focus while pointer access to
   * the host can still select another item; `false` leaves the host fully operable beside the
   * pane (its menus and dialogs included). The overlay always contains focus: it covers the host.
   */
  modal?: "trap-focus" | false;
}

/** A resizable detail pane beside its host; pointer access to the host can select another item. */
export function ResizableDrawer({
  children,
  content,
  title,
  actions,
  open,
  onOpenChange,
  width,
  onWidthChange,
  minWidth = 360,
  maxWidth = 800,
  inlineBreakpoint = 1440,
  breakpointBasis = "viewport",
  header,
  returnFocus,
  initialFocus,
  modal = "trap-focus",
}: ResizableDrawerProps) {
  const [container, setContainer] = useState<HTMLDivElement | null>(null);
  const viewportInline = useInlineLayout(inlineBreakpoint);
  const containerInline = useContainerInlineLayout(container, inlineBreakpoint);
  const contained = breakpointBasis === "container";
  // A contained drawer waits for its host's first measurement rather than mounting its
  // content in one layout and moving it to the other a frame later.
  const inline = contained ? containerInline === true : viewportInline;
  const overlay = contained ? containerInline === false : !viewportInline;
  const drag = useRef<{ x: number; width: number } | null>(null);
  const popup = useRef<HTMLDivElement | null>(null);
  const finalFocus = () => {
    const target = typeof returnFocus === "function" ? returnFocus() : returnFocus;
    return target?.isConnected ? target : true;
  };
  const resize = (next: number) => {
    const maximum = Math.min(maxWidth, (container?.clientWidth ?? maxWidth / 0.7) * 0.7);
    onWidthChange(Math.max(Math.min(minWidth, maximum), Math.min(maximum, next)));
  };
  return (
    <DialogPrimitive.Root
      open={open && inline}
      onOpenChange={onOpenChange}
      modal={modal}
      disablePointerDismissal
    >
      <div ref={setContainer} className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
        <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">{children}</div>
        {open && inline && container && (
          <DialogPrimitive.Portal container={container} className="contents">
            <DialogPrimitive.Popup
              data-slot="resizable-drawer"
              data-mode="inline"
              ref={popup}
              finalFocus={finalFocus}
              initialFocus={initialFocus ?? (() => defaultDialogInitialFocus(popup.current))}
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
                className="absolute inset-y-0 -left-0.5 z-10 w-1 cursor-col-resize touch-none after:absolute after:top-1/2 after:left-1/2 after:h-8 after:w-0.5 after:-translate-x-1/2 after:-translate-y-1/2 after:rounded-full after:bg-line-strong hover:bg-line-strong focus-visible:bg-line-strong focus-visible:outline-none"
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
              {header ? (
                <>
                  <DialogPrimitive.Title className="sr-only">{title}</DialogPrimitive.Title>
                  {header}
                </>
              ) : (
                <header className="flex shrink-0 items-start gap-2 border-b border-line p-4">
                  <DialogPrimitive.Title className="min-w-0 flex-1 text-item-title font-medium text-fg-strong">
                    {title}
                  </DialogPrimitive.Title>
                  {actions}
                  <DialogPrimitive.Close
                    render={<Button variant="quiet" size="icon-sm" aria-label="Close drawer" />}
                  >
                    <XIcon />
                  </DialogPrimitive.Close>
                </header>
              )}
              <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{content}</div>
            </DialogPrimitive.Popup>
          </DialogPrimitive.Portal>
        )}
      </div>
      <Sheet open={open && overlay} onOpenChange={onOpenChange} modal="trap-focus">
        <SheetContent
          data-slot="resizable-drawer"
          data-mode="overlay"
          finalFocus={finalFocus}
          initialFocus={initialFocus}
          showCloseButton={!header}
          className="w-full gap-0 sm:max-w-none"
          style={{ width: contained ? "100%" : `min(${width}px, 100%)` }}
        >
          {header ? (
            <>
              <SheetTitle className="sr-only">{title}</SheetTitle>
              {header}
            </>
          ) : (
            <SheetHeader className="shrink-0 border-b border-line pr-12">
              <div className="flex min-w-0 items-start gap-2">
                <SheetTitle className="min-w-0 flex-1">{title}</SheetTitle>
                {actions}
              </div>
            </SheetHeader>
          )}
          <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{content}</div>
        </SheetContent>
      </Sheet>
    </DialogPrimitive.Root>
  );
}
