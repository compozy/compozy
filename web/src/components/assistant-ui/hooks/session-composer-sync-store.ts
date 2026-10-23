import { createStoreLogic } from "@xstate/store";

interface SessionComposerSyncState {
  hydratedDraftText: string | null;
  hydratedSessionId: string | null;
  suppressedComposerText: string | null;
  preHydrationComposerText: string | null;
  /** The draft owner whose retained attachments this composer already took back. */
  attachmentsMountedSessionId: string | null;
  /** True while restored attachments are re-entering the runtime; observations are partial. */
  restoringAttachments: boolean;
}

type SessionComposerSyncEvents = {
  attachmentDraftMounted: {
    sessionId: string;
    /** Retained files of this draft, in composer order. */
    retainedFiles: readonly File[];
    /** Attachments the rebuilt composer already holds (a recovered send, a first prompt). */
    composerAttachmentCount: number;
    restore: (files: readonly File[]) => Promise<unknown>;
    settled: () => void;
  };
  attachmentRestoreSettled: {};
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
    preHydrationComposerText: null,
    attachmentsMountedSessionId: null,
    restoringAttachments: false,
  },
  on: {
    attachmentDraftMounted: (context, event, enqueue) => {
      // A re-run effect for the same composer must not add the files twice.
      if (context.attachmentsMountedSessionId === event.sessionId) return;
      const restore = event.retainedFiles.length > 0 && event.composerAttachmentCount === 0;
      if (restore) {
        enqueue.effect(() => {
          void Promise.resolve()
            .then(() => event.restore(event.retainedFiles))
            .catch(error =>
              console.error("Failed to restore the session composer attachments", error)
            )
            .finally(event.settled);
        });
      }
      return {
        ...context,
        attachmentsMountedSessionId: event.sessionId,
        restoringAttachments: restore,
      };
    },
    attachmentRestoreSettled: context => {
      if (!context.restoringAttachments) return;
      return { ...context, restoringAttachments: false };
    },
    draftObserved: (context, event, enqueue) => {
      const sameSession = context.hydratedSessionId === event.sessionId;
      if (sameSession && event.composerText !== context.hydratedDraftText) return;
      if (sameSession && context.hydratedDraftText === event.draftText) return;
      enqueue.effect(() =>
        invoke(() => event.hydrate(event.draftText), "Failed to hydrate the session composer")
      );
      return {
        ...context,
        hydratedDraftText: event.draftText,
        hydratedSessionId: event.sessionId,
        suppressedComposerText: event.draftText,
        preHydrationComposerText: event.composerText,
      };
    },
    composerObserved: (context, event, enqueue) => {
      // assistant-ui publishes setText on its next commit; stale observations must not
      // overwrite the persisted draft while that hydration is still pending.
      if (context.suppressedComposerText !== null) {
        if (context.suppressedComposerText === event.composerText) {
          return { ...context, suppressedComposerText: null, preHydrationComposerText: null };
        }
        if (context.preHydrationComposerText === event.composerText) return;
      }
      if (event.composerText === event.draftText) {
        return { ...context, suppressedComposerText: null, preHydrationComposerText: null };
      }
      enqueue.effect(() =>
        invoke(
          () => event.persist(event.composerText),
          "Failed to persist the session composer draft"
        )
      );
      return { ...context, suppressedComposerText: null, preHydrationComposerText: null };
    },
  },
});
