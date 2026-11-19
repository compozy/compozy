import { useState } from "react";

import type { WorkspaceFolderPayload } from "../types";
import type { SessionFolderDialogState } from "./use-session-list-folder-layout";

/**
 * A session list's own UI state: the local filter of a list without a remote search, and the
 * project folder dialog. The dialog is held under the lens it opened in (`lens`): a profile or
 * scope change drops it, so it never submits a folder from the old profile.
 */
export function useSessionListLocalState(lens: string) {
  const [filter, setFilter] = useState("");
  const [dialogState, setDialogState] = useState<{
    lens: string;
    dialog: SessionFolderDialogState<WorkspaceFolderPayload>;
  }>({ lens, dialog: null });
  return {
    filter,
    setFilter,
    projectFolderDialog: dialogState.lens === lens ? dialogState.dialog : null,
    setProjectFolderDialog: (dialog: SessionFolderDialogState<WorkspaceFolderPayload>) =>
      setDialogState({ lens, dialog }),
  };
}
