import { createStoreLogic } from "@xstate/store";

export type TaskStreamState = "idle" | "receiving" | "error" | "disabled";

interface TaskStreamStatusContext {
  error: string | null;
  state: TaskStreamState;
}

type TaskStreamStatusEvents = {
  frameReceived: Record<never, never>;
  streamFailed: { error: string };
};

export const taskStreamStatusLogic = createStoreLogic<
  TaskStreamStatusContext,
  TaskStreamStatusEvents,
  never,
  { enabled: boolean }
>({
  context: input => ({
    error: null,
    state: input.enabled ? "idle" : "disabled",
  }),
  on: {
    frameReceived: context => {
      if (context.state === "disabled") return;
      return { error: null, state: "receiving" };
    },
    streamFailed: (context, event) => {
      if (context.state === "disabled") return;
      return { error: event.error, state: "error" };
    },
  },
});
