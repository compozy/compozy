import { createStoreLogic } from "@xstate/store";

import type { SessionBusyInputDraft, SessionBusyInputHandler } from "@/systems/session";

interface SessionBusyInputState {
  editingQueuedPromptId: string | null;
  phase: "idle" | "submitting";
}

type SessionBusyInputEvents = {
  editCompleted: Record<never, never>;
  editStarted: { id: string };
  promptRemoved: { id: string };
  submissionFinished: Record<never, never>;
  submissionRequested: {
    canSubmit: boolean;
    clearComposer: (options?: { retainAttachments?: boolean }) => void;
    handler: SessionBusyInputHandler | undefined;
    draft: SessionBusyInputDraft;
    onFailure: (error: unknown) => void;
    onSuccess?: () => void;
  };
};

export const sessionBusyInputLogic = createStoreLogic<
  SessionBusyInputState,
  SessionBusyInputEvents
>({
  context: { editingQueuedPromptId: null, phase: "idle" },
  on: {
    editStarted: (context, event) => ({ ...context, editingQueuedPromptId: event.id }),
    editCompleted: context => ({ ...context, editingQueuedPromptId: null }),
    promptRemoved: (context, event) =>
      context.editingQueuedPromptId === event.id
        ? { ...context, editingQueuedPromptId: null }
        : undefined,
    submissionRequested: (context, event, enqueue) => {
      const { handler } = event;
      if (!event.canSubmit || !handler || context.phase === "submitting") return;
      enqueue.effect(async ({ trigger }) => {
        try {
          await handler(event.draft);
          event.clearComposer({ retainAttachments: event.draft.attachments.length > 0 });
          event.onSuccess?.();
        } catch (error) {
          event.onFailure(error);
        } finally {
          trigger.submissionFinished();
        }
      });
      return { ...context, phase: "submitting" };
    },
    submissionFinished: context =>
      context.phase === "idle" ? undefined : { ...context, phase: "idle" },
  },
});
