import { useState } from "react";

export interface UseSessionRenameDialogResult {
  open: boolean;
  setOpen: (open: boolean) => void;
  openDialog: () => void;
  confirmRename: (name: string) => void;
}

/** Owns the detail-window rename dialog state and closes it after a successful rename. */
export function useSessionRenameDialog(
  onRename: (name: string) => Promise<void>
): UseSessionRenameDialogResult {
  const [open, setOpen] = useState(false);

  const confirmRename = (name: string) => {
    void onRename(name)
      .then(() => setOpen(false))
      .catch(() => undefined);
  };

  return {
    open,
    setOpen,
    openDialog: () => setOpen(true),
    confirmRename,
  };
}
