"use client";

import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import * as React from "react";

import { cn } from "../lib/utils";
import { Button } from "./button";
import {
  DialogMotionContext,
  type DialogMotionContextValue,
  useDialogMotion,
} from "./hooks/use-dialog-motion";
import { useDialogMotionTransition } from "./hooks/use-dialog-motion-transition";
import { useInitialState } from "./use-initial-state";
import { useOverlayContainer } from "./hooks/use-overlay-container";

type DialogRootProps = DialogPrimitive.Root.Props;

function Dialog({
  open: controlledOpen,
  defaultOpen = false,
  modal,
  disablePointerDismissal,
  onOpenChange,
  children,
  ...props
}: DialogRootProps) {
  const overlayContainer = useOverlayContainer();
  const windowScoped = overlayContainer !== null;
  const actionsRef = React.useRef<DialogPrimitive.Root.Actions | null>(null);
  const [uncontrolledOpen, setUncontrolledOpen] = useInitialState(defaultOpen);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? Boolean(controlledOpen) : uncontrolledOpen;

  const handleOpenChange: NonNullable<DialogRootProps["onOpenChange"]> = (next, details) => {
    if (!isControlled) setUncontrolledOpen(next);
    onOpenChange?.(next, details);
  };

  const value: DialogMotionContextValue = { actionsRef, open };

  return (
    <DialogPrimitive.Root
      data-slot="dialog"
      actionsRef={actionsRef}
      open={open}
      defaultOpen={defaultOpen}
      modal={modal ?? (windowScoped ? false : true)}
      disablePointerDismissal={disablePointerDismissal ?? windowScoped}
      onOpenChange={handleOpenChange}
      {...props}
    >
      <DialogMotionContext.Provider value={value}>
        {children as React.ReactNode}
      </DialogMotionContext.Provider>
    </DialogPrimitive.Root>
  );
}

function DialogTrigger({ ...props }: DialogPrimitive.Trigger.Props) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />;
}

function DialogPortal({ container, ...props }: DialogPrimitive.Portal.Props) {
  const overlayContainer = useOverlayContainer();
  return (
    <DialogPrimitive.Portal
      data-slot="dialog-portal"
      container={container !== undefined ? container : (overlayContainer ?? undefined)}
      {...props}
    />
  );
}

function DialogClose({ ...props }: DialogPrimitive.Close.Props) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />;
}

function DialogOverlay({ className, style, ...props }: DialogPrimitive.Backdrop.Props) {
  const transition = useDialogMotionTransition();
  const overlayRender = (
    <m.div initial={false} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={transition} />
  );
  return (
    <DialogPrimitive.Backdrop
      data-slot="dialog-overlay"
      render={overlayRender}
      className={cn("fixed inset-0 isolate z-50 bg-overlay-scrim", className)}
      style={{ backdropFilter: "blur(var(--overlay-blur))", ...style }}
      {...props}
    />
  );
}

type DialogChromeVariant = "default" | "ruled";

const DIALOG_CONTENT_BASE =
  "fixed top-1/2 left-1/2 z-50 grid w-full max-w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-canvas-soft text-small-body text-fg shadow-overlay outline-none sm:max-w-sm";
const DIALOG_CONTENT_FRAMED = "gap-4 p-4";
const DIALOG_CONTENT_UNFRAMED = "gap-0 p-0";

const DIALOG_HEADER_DEFAULT = "flex flex-col gap-2";
// The 20px gutter is shared by the mode toolbar, body, and footer; do not align
// it to the 24px route gutter, which belongs to page content.
const DIALOG_HEADER_RULED = "flex flex-col gap-2 border-b border-line bg-canvas-soft px-5 py-4";

const DIALOG_FOOTER_DEFAULT =
  "-mx-4 -mb-4 flex flex-col-reverse gap-2 rounded-b-lg border-t border-line bg-canvas-tint p-4 sm:flex-row sm:justify-end";
// The ruled footer uses the same horizontal gutter as the rest of the shell.
const DIALOG_FOOTER_RULED =
  "flex flex-col-reverse gap-2 border-t border-line bg-canvas-tint px-5 py-3 sm:flex-row sm:justify-end";

interface DialogContentProps extends DialogPrimitive.Popup.Props {
  showCloseButton?: boolean;
  /**
   * When `true`, drops the default `gap-4 p-4` chrome so callers can compose
   * flush headers, bodies, and footers (typically alongside `DialogHeader`/
   * `DialogFooter` `variant="ruled"`).
   */
  unframed?: boolean;
}

function DialogContent({
  className,
  children,
  showCloseButton = true,
  unframed = false,
  style,
  ...props
}: DialogContentProps) {
  const { actionsRef, open } = useDialogMotion();
  const overlayContainer = useOverlayContainer();
  const windowScoped = overlayContainer !== null;
  const transition = useDialogMotionTransition();
  const popupRender = (
    <m.div
      initial={false}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.97 }}
      transition={transition}
    />
  );

  const handleExitComplete = () => {
    actionsRef.current?.unmount();
  };

  return (
    <AnimatePresence onExitComplete={handleExitComplete}>
      {open ? (
        <DialogPortal key="dialog-portal" keepMounted>
          <DialogOverlay />
          <DialogPrimitive.Popup
            data-slot="dialog-content"
            data-frame={unframed ? "unframed" : "framed"}
            render={popupRender}
            className={cn(
              DIALOG_CONTENT_BASE,
              unframed ? DIALOG_CONTENT_UNFRAMED : DIALOG_CONTENT_FRAMED,
              unframed && "overflow-hidden",
              className
            )}
            style={{
              ...(windowScoped
                ? {
                    maxHeight: "calc(100% - 2rem)",
                    maxWidth: "calc(100% - 2rem)",
                  }
                : undefined),
              ...style,
            }}
            {...props}
          >
            {children}
            {showCloseButton ? (
              <DialogPrimitive.Close
                data-slot="dialog-close"
                render={
                  <Button variant="ghost" className="absolute top-2 right-2" size="icon-sm" />
                }
              >
                <XIcon />
                <span className="sr-only">Close</span>
              </DialogPrimitive.Close>
            ) : null}
          </DialogPrimitive.Popup>
        </DialogPortal>
      ) : null}
    </AnimatePresence>
  );
}

interface DialogHeaderProps extends React.ComponentProps<"div"> {
  variant?: DialogChromeVariant;
}

function DialogHeader({ className, variant = "default", ...props }: DialogHeaderProps) {
  return (
    <div
      data-slot="dialog-header"
      data-variant={variant}
      className={cn(variant === "ruled" ? DIALOG_HEADER_RULED : DIALOG_HEADER_DEFAULT, className)}
      {...props}
    />
  );
}

interface DialogFooterProps extends React.ComponentProps<"div"> {
  showCloseButton?: boolean;
  variant?: DialogChromeVariant;
}

function DialogFooter({
  className,
  showCloseButton = false,
  variant = "default",
  children,
  ...props
}: DialogFooterProps) {
  return (
    <div
      data-slot="dialog-footer"
      data-variant={variant}
      className={cn(variant === "ruled" ? DIALOG_FOOTER_RULED : DIALOG_FOOTER_DEFAULT, className)}
      {...props}
    >
      {children}
      {showCloseButton ? (
        <DialogClose render={<Button variant="outline" />}>Close</DialogClose>
      ) : null}
    </div>
  );
}

function DialogTitle({ className, ...props }: DialogPrimitive.Title.Props) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn(
        "text-item-title leading-none font-medium tracking-tight text-fg-strong",
        className
      )}
      {...props}
    />
  );
}

function DialogDescription({ className, ...props }: DialogPrimitive.Description.Props) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn(
        "text-small-body text-muted *:[a]:underline *:[a]:underline-offset-3 *:[a]:hover:text-fg-strong",
        className
      )}
      {...props}
    />
  );
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
};
export type { DialogChromeVariant, DialogContentProps, DialogFooterProps, DialogHeaderProps };
