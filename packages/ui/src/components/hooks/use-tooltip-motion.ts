import { Tooltip as TooltipPrimitive } from "@base-ui/react/tooltip";
import * as React from "react";

export type TooltipActionsRef = React.RefObject<TooltipPrimitive.Root.Actions | null>;

export interface TooltipMotionContextValue {
  actionsRef: TooltipActionsRef;
  open: boolean;
}

export const TooltipMotionContext = React.createContext<TooltipMotionContextValue | null>(null);

export function useTooltipMotion(): TooltipMotionContextValue {
  const ctx = React.use(TooltipMotionContext);
  if (!ctx) {
    throw new Error("Tooltip.* components must be used inside <Tooltip>.");
  }
  return ctx;
}
