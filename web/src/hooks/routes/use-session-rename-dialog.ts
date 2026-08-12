import { useState } from "react";

export interface UseSessionRenameDialogResult {
  error: string | null;
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
  const [error, setError] = useState<string | null>(null);

  const setDialogOpen = (nextOpen: boolean) => {
    setError(null);
    setOpen(nextOpen);
  };

  const confirmRename = (name: string) => {
    setError(null);
    void (async () => {
      try {
        await onRename(name);
        setOpen(false);
      } catch (renameError) {
        setError(
          renameError instanceof Error && renameError.message.trim().length > 0
            ? renameError.message
            : "Failed to rename session."
        );
      }
    })();
  };

  return {
    error,
    open,
    setOpen: setDialogOpen,
    openDialog: () => setDialogOpen(true),
    confirmRename,
  };
}
