"use client";

import * as React from "react";
import { Tooltip as TooltipPrimitive } from "@base-ui/react/tooltip";
import { AnimatePresence, m } from "motion/react";

import { MOTION_DURATION_BASE, MOTION_EASE_OUT } from "../lib/motion";
import { cn } from "../lib/utils";
import {
  TooltipMotionContext,
  type TooltipMotionContextValue,
  useTooltipMotion,
} from "./hooks/use-tooltip-motion";
import { useInitialState } from "./use-initial-state";

function TooltipProvider({ delay = 0, ...props }: TooltipPrimitive.Provider.Props) {
  return <TooltipPrimitive.Provider data-slot="tooltip-provider" delay={delay} {...props} />;
}

type TooltipRootProps = TooltipPrimitive.Root.Props;

function Tooltip({
  open: controlledOpen,
  defaultOpen = false,
  onOpenChange,
  children,
  ...props
}: TooltipRootProps) {
  const actionsRef = React.useRef<TooltipPrimitive.Root.Actions | null>(null);
  const [uncontrolledOpen, setUncontrolledOpen] = useInitialState(defaultOpen);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? Boolean(controlledOpen) : uncontrolledOpen;

  const handleOpenChange: NonNullable<TooltipRootProps["onOpenChange"]> = (next, details) => {
    if (!isControlled) setUncontrolledOpen(next);
    onOpenChange?.(next, details);
  };

  const value: TooltipMotionContextValue = { actionsRef, open };

  return (
    <TooltipPrimitive.Root
      data-slot="tooltip"
      actionsRef={actionsRef}
      open={open}
      defaultOpen={defaultOpen}
      onOpenChange={handleOpenChange}
      {...props}
    >
      <TooltipMotionContext.Provider value={value}>
        {children as React.ReactNode}
      </TooltipMotionContext.Provider>
    </TooltipPrimitive.Root>
  );
}

function TooltipTrigger({ ...props }: TooltipPrimitive.Trigger.Props) {
  return <TooltipPrimitive.Trigger data-slot="tooltip-trigger" {...props} />;
}

type TooltipContentProps = TooltipPrimitive.Popup.Props &
  Pick<TooltipPrimitive.Positioner.Props, "align" | "alignOffset" | "side" | "sideOffset">;

function TooltipContent({
  className,
  side = "top",
  sideOffset = 4,
  align = "center",
  alignOffset = 0,
  children,
  ...props
}: TooltipContentProps) {
  const { actionsRef, open } = useTooltipMotion();

  const handleExitComplete = () => {
    actionsRef.current?.unmount();
  };

  return (
    <AnimatePresence onExitComplete={handleExitComplete}>
      {open ? (
        <TooltipPrimitive.Portal key="tooltip-portal" keepMounted>
          <TooltipPrimitive.Positioner
            align={align}
            alignOffset={alignOffset}
            side={side}
            sideOffset={sideOffset}
            className="isolate z-50"
          >
            <TooltipPrimitive.Popup
              data-slot="tooltip-content"
              render={
                <m.div
                  initial={{ opacity: 0, scale: 0.95 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.95 }}
                  transition={{ duration: MOTION_DURATION_BASE, ease: MOTION_EASE_OUT }}
                />
              }
              className={cn(
                "z-50 inline-flex w-fit max-w-xs origin-(--transform-origin) items-center gap-1.5 rounded-md bg-canvas-soft px-3 py-1.5 text-form-label text-fg-strong shadow-hairline has-data-[slot=kbd]:pr-1.5 **:data-[slot=kbd]:relative **:data-[slot=kbd]:isolate **:data-[slot=kbd]:z-50 **:data-[slot=kbd]:rounded-xs",
                className
              )}
              {...props}
            >
              {children}
              <TooltipPrimitive.Arrow className="z-50 size-2.5 translate-y-[calc(-50%-var(--space-switch-thumb-inset))] rotate-45 rounded-xs bg-canvas-soft fill-canvas-soft data-[side=bottom]:top-1 data-[side=inline-end]:top-1/2! data-[side=inline-end]:-left-1 data-[side=inline-end]:-translate-y-1/2 data-[side=inline-start]:top-1/2! data-[side=inline-start]:-right-1 data-[side=inline-start]:-translate-y-1/2 data-[side=left]:top-1/2! data-[side=left]:-right-1 data-[side=left]:-translate-y-1/2 data-[side=right]:top-1/2! data-[side=right]:-left-1 data-[side=right]:-translate-y-1/2 data-[side=top]:-bottom-2.5" />
            </TooltipPrimitive.Popup>
          </TooltipPrimitive.Positioner>
        </TooltipPrimitive.Portal>
      ) : null}
    </AnimatePresence>
  );
}

export { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider };
