import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import * as React from "react";

export type DialogActionsRef = React.RefObject<DialogPrimitive.Root.Actions | null>;

export interface DialogMotionContextValue {
  actionsRef: DialogActionsRef;
  open: boolean;
}

export const DialogMotionContext = React.createContext<DialogMotionContextValue | null>(null);

export function useDialogMotion(): DialogMotionContextValue {
  const ctx = React.use(DialogMotionContext);
  if (!ctx) {
    throw new Error("Dialog.* components must be used inside <Dialog>.");
  }
  return ctx;
}
