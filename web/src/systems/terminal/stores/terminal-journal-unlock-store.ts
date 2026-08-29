import { createStoreLogic } from "@xstate/store";

interface TerminalJournalUnlockState {
  unlockedWorkspaces: Record<string, true>;
}

type TerminalJournalUnlockEvents = {
  journalOpened: { workspaceId: string };
};

export const terminalJournalUnlockLogic = createStoreLogic<
  TerminalJournalUnlockState,
  TerminalJournalUnlockEvents
>({
  context: { unlockedWorkspaces: {} },
  on: {
    journalOpened: (context, event) =>
      context.unlockedWorkspaces[event.workspaceId]
        ? undefined
        : {
            unlockedWorkspaces: {
              ...context.unlockedWorkspaces,
              [event.workspaceId]: true,
            },
          },
  },
});

export const terminalJournalUnlockStore = terminalJournalUnlockLogic.createStore();

export function unlockTerminalJournal(workspaceId: string): void {
  if (workspaceId === "") return;
  terminalJournalUnlockStore.trigger.journalOpened({ workspaceId });
}
