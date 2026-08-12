import { createStoreLogic } from "@xstate/store";

interface SessionBusyInputState {
  editingQueuedPromptId: string | null;
  phase: "idle" | "submitting";
}

type SessionBusyInputEvents = {
  editCompleted: Record<never, never>;
  editStarted: { id: string };
  promptRemoved: { id: string };
  submissionFinished: Record<never, never>;
  submissionStarted: Record<never, never>;
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
    submissionStarted: context =>
      context.phase === "submitting" ? undefined : { ...context, phase: "submitting" },
    submissionFinished: context =>
      context.phase === "idle" ? undefined : { ...context, phase: "idle" },
  },
});
