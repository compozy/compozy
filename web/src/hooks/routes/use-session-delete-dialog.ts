import { useState } from "react";

export interface UseSessionDeleteDialogResult {
  open: boolean;
  setOpen: (open: boolean) => void;
  openDialog: () => void;
  confirmDelete: () => void;
}

/**
 * Bundles the session-detail delete-confirmation dialog's open state and
 * confirm handler so the route component stays under the
 * `compozy-react/max-component-complexity` hook ceiling.
 */
export function useSessionDeleteDialog(onDelete: () => void): UseSessionDeleteDialogResult {
  const [open, setOpen] = useState(false);
  const openDialog = () => setOpen(true);
  const confirmDelete = () => {
    setOpen(false);
    onDelete();
  };
  return { open, setOpen, openDialog, confirmDelete };
}
