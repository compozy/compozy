import { createStoreLogic } from "@xstate/store";

interface SessionComposerSyncState {
  hydratedDraftText: string | null;
  hydratedSessionId: string | null;
  suppressedComposerText: string | null;
}

type SessionComposerSyncEvents = {
  composerObserved: {
    composerText: string;
    draftText: string;
    persist: (text: string) => void;
  };
  draftObserved: {
    composerText: string;
    draftText: string;
    hydrate: (text: string) => void;
    sessionId: string;
  };
};

function invoke(operation: () => void, label: string): void {
  try {
    operation();
  } catch (error) {
    console.error(label, error);
  }
}

export const sessionComposerSyncLogic = createStoreLogic<
  SessionComposerSyncState,
  SessionComposerSyncEvents
>({
  context: {
    hydratedDraftText: null,
    hydratedSessionId: null,
    suppressedComposerText: null,
  },
  on: {
    draftObserved: (context, event, enqueue) => {
      const sameSession = context.hydratedSessionId === event.sessionId;
      if (sameSession && event.composerText !== context.hydratedDraftText) return;
      if (sameSession && context.hydratedDraftText === event.draftText) return;
      enqueue.effect(() =>
        invoke(() => event.hydrate(event.draftText), "Failed to hydrate the session composer")
      );
      return {
        hydratedDraftText: event.draftText,
        hydratedSessionId: event.sessionId,
        suppressedComposerText: event.draftText,
      };
    },
    composerObserved: (context, event, enqueue) => {
      if (context.suppressedComposerText === event.composerText) {
        return { ...context, suppressedComposerText: null };
      }
      if (event.composerText === event.draftText) {
        return context.suppressedComposerText === null
          ? undefined
          : { ...context, suppressedComposerText: null };
      }
      enqueue.effect(() =>
        invoke(
          () => event.persist(event.composerText),
          "Failed to persist the session composer draft"
        )
      );
      return { ...context, suppressedComposerText: null };
    },
  },
});
