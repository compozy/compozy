import { useSelector } from "@xstate/store-react";

import { terminalJournalUnlockStore } from "../stores/terminal-journal-unlock-store";

/** Keeps first-open lazy loading stable when the OS rematerializes a window. */
export function useTerminalJournalUnlocked(workspaceId: string): boolean {
  return useSelector(
    terminalJournalUnlockStore,
    snapshot => workspaceId !== "" && snapshot.context.unlockedWorkspaces[workspaceId] === true
  );
}
