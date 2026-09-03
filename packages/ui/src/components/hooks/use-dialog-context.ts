import * as React from "react";

export interface DialogContextValue {
  open: boolean;
}

export const DialogContext = React.createContext<DialogContextValue | null>(null);

export function useDialogContext(): DialogContextValue {
  const context = React.use(DialogContext);
  if (context === null) {
    throw new Error("Dialog.* components must be used inside <Dialog>.");
  }
  return context;
}
