"use client";

import { Dialog as SheetPrimitive } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import { AnimatePresence, m, type Variants } from "motion/react";
import * as React from "react";

import { MOTION_DURATION_SLOW, MOTION_EASE_IN_OUT, MOTION_EASE_OUT } from "../lib/motion";
import { cn } from "../lib/utils";
import { Button } from "./button";
import {
  SheetMotionContext,
  type SheetMotionContextValue,
  useSheetMotion,
} from "./hooks/use-sheet-motion";
import { useInitialState } from "./use-initial-state";
import { useOverlayContainer } from "./hooks/use-overlay-container";

type SheetSide = "top" | "right" | "bottom" | "left";

type SheetRootProps = SheetPrimitive.Root.Props;

function Sheet({
  open: controlledOpen,
  defaultOpen = false,
  modal,
  disablePointerDismissal,
  onOpenChange,
  children,
  ...props
}: SheetRootProps) {
  const overlayContainer = useOverlayContainer();
  const windowScoped = overlayContainer !== null;
  const actionsRef = React.useRef<SheetPrimitive.Root.Actions | null>(null);
  const [uncontrolledOpen, setUncontrolledOpen] = useInitialState(defaultOpen);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? Boolean(controlledOpen) : uncontrolledOpen;

  const handleOpenChange: NonNullable<SheetRootProps["onOpenChange"]> = (next, details) => {
    if (!isControlled) setUncontrolledOpen(next);
    onOpenChange?.(next, details);
  };

  const value: SheetMotionContextValue = { actionsRef, open };

  return (
    <SheetPrimitive.Root
      data-slot="sheet"
      actionsRef={actionsRef}
      open={open}
      defaultOpen={defaultOpen}
      modal={modal ?? (windowScoped ? false : true)}
      disablePointerDismissal={disablePointerDismissal ?? windowScoped}
      onOpenChange={handleOpenChange}
      {...props}
    >
      <SheetMotionContext.Provider value={value}>
        {children as React.ReactNode}
      </SheetMotionContext.Provider>
    </SheetPrimitive.Root>
  );
}

function SheetTrigger({ ...props }: SheetPrimitive.Trigger.Props) {
  return <SheetPrimitive.Trigger data-slot="sheet-trigger" {...props} />;
}

function SheetClose({ ...props }: SheetPrimitive.Close.Props) {
  return <SheetPrimitive.Close data-slot="sheet-close" {...props} />;
}

function SheetPortal({ container, ...props }: SheetPrimitive.Portal.Props) {
  const overlayContainer = useOverlayContainer();
  return (
    <SheetPrimitive.Portal
      data-slot="sheet-portal"
      container={container !== undefined ? container : (overlayContainer ?? undefined)}
      {...props}
    />
  );
}

function SheetOverlay({ className, ...props }: SheetPrimitive.Backdrop.Props) {
  return (
    <SheetPrimitive.Backdrop
      data-slot="sheet-overlay"
      render={
        <m.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: MOTION_DURATION_SLOW, ease: MOTION_EASE_OUT }}
        />
      }
      className={cn("fixed inset-0 z-50 bg-overlay-scrim", className)}
      {...props}
    />
  );
}

const SIDE_VARIANTS: Record<SheetSide, Variants> = {
  top: {
    hidden: { opacity: 0, y: "-2.5rem" },
    visible: { opacity: 1, y: 0 },
  },
  bottom: {
    hidden: { opacity: 0, y: "2.5rem" },
    visible: { opacity: 1, y: 0 },
  },
  left: {
    hidden: { opacity: 0, x: "-2.5rem" },
    visible: { opacity: 1, x: 0 },
  },
  right: {
    hidden: { opacity: 0, x: "2.5rem" },
    visible: { opacity: 1, x: 0 },
  },
};

const SIDE_CLASSES: Record<SheetSide, string> = {
  top: "inset-x-0 top-0 h-auto rounded-b-xl",
  bottom: "inset-x-0 bottom-0 h-auto rounded-t-xl",
  left: "inset-y-0 left-0 h-full w-3/4 rounded-r-xl sm:max-w-sm",
  right: "inset-y-0 right-0 h-full w-3/4 rounded-l-xl sm:max-w-sm",
};

interface SheetContentProps extends SheetPrimitive.Popup.Props {
  side?: SheetSide;
  showCloseButton?: boolean;
}

function SheetContent({
  className,
  children,
  side = "right",
  showCloseButton = true,
  ...props
}: SheetContentProps) {
  const { actionsRef, open } = useSheetMotion();

  const handleExitComplete = () => {
    actionsRef.current?.unmount();
  };

  return (
    <AnimatePresence onExitComplete={handleExitComplete}>
      {open ? (
        <SheetPortal key="sheet-portal" keepMounted>
          <SheetOverlay />
          <SheetPrimitive.Popup
            data-slot="sheet-content"
            data-side={side}
            render={
              <m.div
                variants={SIDE_VARIANTS[side]}
                initial="hidden"
                animate="visible"
                exit="hidden"
                transition={{ duration: MOTION_DURATION_SLOW, ease: MOTION_EASE_IN_OUT }}
              />
            }
            className={cn(
              "fixed z-50 flex flex-col gap-4 bg-canvas-soft bg-clip-padding text-small-body text-fg shadow-overlay outline-none",
              SIDE_CLASSES[side],
              className
            )}
            {...props}
          >
            {children}
            {showCloseButton ? (
              <SheetPrimitive.Close
                data-slot="sheet-close"
                render={
                  <Button variant="ghost" className="absolute top-3 right-3" size="icon-sm" />
                }
              >
                <XIcon />
                <span className="sr-only">Close</span>
              </SheetPrimitive.Close>
            ) : null}
          </SheetPrimitive.Popup>
        </SheetPortal>
      ) : null}
    </AnimatePresence>
  );
}

function SheetHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sheet-header"
      className={cn("flex flex-col gap-0.5 p-4", className)}
      {...props}
    />
  );
}

function SheetFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sheet-footer"
      className={cn("mt-auto flex flex-col gap-2 p-4", className)}
      {...props}
    />
  );
}

function SheetTitle({ className, ...props }: SheetPrimitive.Title.Props) {
  return (
    <SheetPrimitive.Title
      data-slot="sheet-title"
      className={cn("text-item-title font-medium tracking-tight text-fg-strong", className)}
      {...props}
    />
  );
}

function SheetDescription({ className, ...props }: SheetPrimitive.Description.Props) {
  return (
    <SheetPrimitive.Description
      data-slot="sheet-description"
      className={cn("text-small-body text-muted", className)}
      {...props}
    />
  );
}

export {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
};
