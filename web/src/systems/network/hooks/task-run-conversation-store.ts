import { createStoreLogic } from "@xstate/store";

import type { TaskRunNetworkUsage } from "../types";

interface TaskRunConversationState {
  scope: string;
  streamError: Error | null;
  usageOverlay: TaskRunNetworkUsage | null;
}

type TaskRunConversationEvents = {
  frameReceived: { scope: string };
  streamFailed: { error: Error; scope: string };
  streamSuspended: { scope: string };
  usageReceived: { scope: string; usage: TaskRunNetworkUsage };
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
    usageOverlay: null,
  }),
  on: {
    frameReceived: (context, event) => {
      if (event.scope !== context.scope || context.streamError === null) return;
      return { ...context, streamError: null };
    },
    usageReceived: (context, event) => {
      if (event.scope !== context.scope) return;
      return {
        ...context,
        streamError: null,
        usageOverlay: event.usage,
      };
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
