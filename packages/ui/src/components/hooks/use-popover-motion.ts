import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import * as React from "react";

export type PopoverActionsRef = React.RefObject<PopoverPrimitive.Root.Actions | null>;

export interface PopoverMotionContextValue {
  actionsRef: PopoverActionsRef;
  open: boolean;
}

export const PopoverMotionContext = React.createContext<PopoverMotionContextValue | null>(null);

export function usePopoverMotion(): PopoverMotionContextValue {
  const ctx = React.use(PopoverMotionContext);
  if (!ctx) {
    throw new Error("Popover.* components must be used inside <Popover>.");
  }
  return ctx;
}
