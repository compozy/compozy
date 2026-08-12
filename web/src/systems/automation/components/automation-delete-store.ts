import { createStoreLogic } from "@xstate/store";

interface AutomationDeleteState {
  error: string | null;
  open: boolean;
  phase: "idle" | "submitting";
}

type AutomationDeleteEvents = {
  openChanged: { open: boolean };
  submissionFailed: { error: string };
  submissionStarted: Record<never, never>;
  submissionSucceeded: Record<never, never>;
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
    submissionStarted: context =>
      context.phase === "submitting" ? undefined : { ...context, error: null, phase: "submitting" },
    submissionFailed: (context, event) => ({
      ...context,
      error: event.error,
      phase: "idle",
    }),
    submissionSucceeded: context => ({
      ...context,
      error: null,
      open: false,
      phase: "idle",
    }),
  },
});
