import { createStoreLogic } from "@xstate/store";

interface TaskRunConversationState {
  scope: string;
  streamError: Error | null;
}

type TaskRunConversationEvents = {
  frameReceived: { scope: string };
  streamFailed: { error: Error; scope: string };
  streamSuspended: { scope: string };
};

export const taskRunConversationLogic = createStoreLogic<
  TaskRunConversationState,
  TaskRunConversationEvents,
  never,
  { scope: string }
>({
  context: input => ({
    scope: input.scope,
    streamError: null,
  }),
  on: {
    frameReceived: (context, event) => {
      if (event.scope !== context.scope || context.streamError === null) return;
      return { ...context, streamError: null };
    },
    streamFailed: (context, event) => {
      if (event.scope !== context.scope) return;
      return { ...context, streamError: event.error };
    },
    streamSuspended: (context, event) => {
      if (event.scope !== context.scope || context.streamError === null) return;
      return { ...context, streamError: null };
    },
  },
});
