import { useState } from "react";

export interface UseSessionClearDialogResult {
  open: boolean;
  setOpen: (open: boolean) => void;
  openDialog: () => void;
  confirmClear: () => void;
}

/**
 * Bundles the session-detail clear-conversation confirmation dialog's open state
 * and confirm handler so the destructive clear action lives in the topbar (not the
 * composer) while the route component stays under the
 * `compozy-react/max-component-complexity` hook ceiling.
 */
export function useSessionClearDialog(onClear: () => void): UseSessionClearDialogResult {
  const [open, setOpen] = useState(false);
  const openDialog = () => setOpen(true);
  const confirmClear = () => {
    setOpen(false);
    onClear();
  };
  return { open, setOpen, openDialog, confirmClear };
}
