import { createStoreLogic } from "@xstate/store";

import type { TaskExecutionProfileSetRequest } from "../types";

interface ProfileEditorState {
  attemptId: number;
  error: string | null;
  phase: "closed" | "editing" | "submitting";
  value: TaskExecutionProfileSetRequest | null;
}

type ProfileEditorEvents = {
  closed: {};
  opened: { value: TaskExecutionProfileSetRequest };
  submitFailed: { attemptId: number; error: string };
  submitRequested: {
    execute: (value: TaskExecutionProfileSetRequest) => Promise<void>;
    taskId: string;
    updatedAt: string;
  };
  submitSucceeded: { attemptId: number };
  valueChanged: { value: TaskExecutionProfileSetRequest };
};

export function createProfileEditorLogic() {
  return createStoreLogic<ProfileEditorState, ProfileEditorEvents>({
    context: { attemptId: 0, error: null, phase: "closed", value: null },
    on: {
      closed: context => ({ ...context, error: null, phase: "closed", value: null }),
      opened: (context, event) => ({
        ...context,
        error: null,
        phase: "editing",
        value: event.value,
      }),
      submitFailed: (context, event) => {
        if (context.phase !== "submitting" || context.attemptId !== event.attemptId) return;
        return { ...context, error: event.error, phase: "editing" };
      },
      submitRequested: (context, event, enqueue) => {
        if (context.phase !== "editing" || !context.value) return;
        const attemptId = context.attemptId + 1;
        const value = {
          ...context.value,
          task_id: event.taskId,
          updated_at: event.updatedAt,
        };
        enqueue.effect(async ({ trigger }) => {
          try {
            await event.execute(value);
            trigger.submitSucceeded({ attemptId });
          } catch (error) {
            trigger.submitFailed({ attemptId, error: profileErrorMessage(error) });
          }
        });
        return { ...context, attemptId, error: null, phase: "submitting", value };
      },
      submitSucceeded: (context, event) => {
        if (context.phase !== "submitting" || context.attemptId !== event.attemptId) return;
        return { ...context, error: null, phase: "closed", value: null };
      },
      valueChanged: (context, event) => {
        if (context.phase !== "editing") return;
        return { ...context, error: null, value: event.value };
      },
    },
  });
}

function profileErrorMessage(error: unknown): string {
  return error instanceof Error && error.message.trim() !== ""
    ? error.message
    : "Failed to update task profile.";
}
