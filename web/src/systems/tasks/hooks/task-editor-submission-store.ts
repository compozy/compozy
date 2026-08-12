import { createStoreLogic } from "@xstate/store";

interface TaskEditorSubmissionState {
  phase: "idle" | "submitting";
}

type TaskEditorSubmissionEvents = {
  submissionRequested: {
    execute: () => Promise<void>;
    onFailure: (error: unknown) => void;
    onSkipped: () => void;
  };
  submissionFinished: Record<never, never>;
};

/**
 * Submission lifetime belongs to the editor instance, independent of the
 * draft source that can be replaced when its scope or template changes.
 */
export const taskEditorSubmissionLogic = createStoreLogic<
  TaskEditorSubmissionState,
  TaskEditorSubmissionEvents
>({
  context: { phase: "idle" },
  on: {
    submissionRequested: (context, event, enqueue) => {
      if (context.phase === "submitting") {
        enqueue.effect(() => event.onSkipped());
        return context;
      }
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute();
        } catch (error) {
          event.onFailure(error);
        } finally {
          trigger.submissionFinished();
        }
      });
      return { phase: "submitting" };
    },
    submissionFinished: context => {
      if (context.phase === "idle") return;
      return { phase: "idle" };
    },
  },
});

export type TaskEditorSubmissionStore = ReturnType<typeof taskEditorSubmissionLogic.createStore>;

export function requestTaskEditorSubmission<TResult>(
  store: TaskEditorSubmissionStore,
  execute: () => Promise<TResult>,
  onFailure: (error: unknown) => void
): Promise<TResult | null> {
  return new Promise(resolve => {
    store.trigger.submissionRequested({
      execute: async () => {
        resolve(await execute());
      },
      onFailure: error => {
        onFailure(error);
        resolve(null);
      },
      onSkipped: () => resolve(null),
    });
  });
}
