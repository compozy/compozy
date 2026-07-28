import { Dialog as SheetPrimitive } from "@base-ui/react/dialog";
import * as React from "react";

export type SheetActionsRef = React.RefObject<SheetPrimitive.Root.Actions | null>;

export interface SheetMotionContextValue {
  actionsRef: SheetActionsRef;
  open: boolean;
}

export const SheetMotionContext = React.createContext<SheetMotionContextValue | null>(null);

export function useSheetMotion(): SheetMotionContextValue {
  const ctx = React.use(SheetMotionContext);
  if (!ctx) {
    throw new Error("Sheet.* components must be used inside <Sheet>.");
  }
  return ctx;
}
