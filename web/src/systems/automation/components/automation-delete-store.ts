import { createStoreLogic } from "@xstate/store";

interface AutomationDeleteState {
  error: string | null;
  open: boolean;
  phase: "idle" | "submitting";
}

type AutomationDeleteEvents = {
  openChanged: { open: boolean };
  submissionFailed: { error: string };
  submissionRequested: {
    execute: () => void | Promise<void>;
    fallbackError: string;
    permitted: boolean;
    onSucceeded?: () => void;
  };
  submissionSucceeded: { onSucceeded?: () => void };
};

export const automationDeleteLogic = createStoreLogic<
  AutomationDeleteState,
  AutomationDeleteEvents
>({
  context: { error: null, open: false, phase: "idle" },
  on: {
    openChanged: (context, event) => ({
      ...context,
      error: event.open ? context.error : null,
      open: event.open,
    }),
    submissionRequested: (context, event, enqueue) => {
      if (!event.permitted || context.phase === "submitting") return;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute();
          trigger.submissionSucceeded({ onSucceeded: event.onSucceeded });
        } catch (cause) {
          trigger.submissionFailed({
            error: cause instanceof Error ? cause.message : event.fallbackError,
          });
        }
      });
      return { ...context, error: null, phase: "submitting" };
    },
    submissionFailed: (context, event) => ({
      ...context,
      error: event.error,
      phase: "idle",
    }),
    submissionSucceeded: (context, event, enqueue) => {
      enqueue.effect(() => event.onSucceeded?.());
      return {
        ...context,
        error: null,
        open: false,
        phase: "idle",
      };
    },
  },
});
