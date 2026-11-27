import { useState } from "react";

import { buildSessionListThreads } from "../lib/session-list-threads";
import type { SessionPayload, WorkspaceFolderPayload } from "../types";
import type { SessionFolderDialogState } from "./use-session-list-folder-layout";
import type { SessionListViewModel } from "./use-session-list-view";
import { useSubagentContextSessions } from "./use-subagent-context-sessions";

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

export function useSessionListSearch(
  view: Pick<SessionListViewModel, "search" | "selectionKey">,
  sessions: readonly SessionPayload[],
  revealedSession: SessionPayload | null | undefined
) {
  const local = useSessionListLocalState(view.selectionKey);
  const remoteSearch = view.search !== undefined;
  const activeSearch = view.search ?? local.filter;
  const normalizedFilter = activeSearch.trim().toLocaleLowerCase();
  const listed = useSubagentContextSessions(
    sessions,
    revealedSession,
    remoteSearch && normalizedFilter !== ""
  );
  const threads = buildSessionListThreads(listed, remoteSearch ? "" : normalizedFilter);

  return { ...local, activeSearch, normalizedFilter, listed, threads };
}
