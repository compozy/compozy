import { createContext } from "react";
import { createStoreLogic } from "@xstate/store";

interface SessionPromptDispatchContext {
  canceled: boolean;
  controller: AbortController | null;
  generation: number;
  hasLocalRuntimeTail: boolean;
  pending: boolean;
  streamResetGeneration: number;
}

export type SessionPromptDispatchStoreEvents = {
  conversationReset: Record<never, never>;
  pendingCanceled: Record<never, never>;
  requestCompleted: { controller: AbortController };
  requestStarted: { controller: AbortController };
};

export const sessionPromptDispatchLogic = createStoreLogic<
  SessionPromptDispatchContext,
  SessionPromptDispatchStoreEvents
>({
  context: {
    canceled: false,
    controller: null,
    generation: 0,
    hasLocalRuntimeTail: false,
    pending: false,
    streamResetGeneration: 0,
  },
  on: {
    conversationReset: context => ({
      ...context,
      hasLocalRuntimeTail: false,
      streamResetGeneration: context.streamResetGeneration + 1,
    }),
    pendingCanceled: (context, _event, enqueue) => {
      if (context.controller === null) return;
      const controller = context.controller;
      enqueue.effect(() => controller.abort());
      return {
        ...context,
        canceled: true,
        controller: null,
        generation: context.generation + 1,
        pending: false,
      };
    },
    requestCompleted: (context, event) => {
      if (context.controller !== event.controller) return;
      return {
        ...context,
        canceled: false,
        controller: null,
        pending: false,
      };
    },
    requestStarted: (context, event, enqueue) => {
      if (context.controller !== null && context.controller !== event.controller) {
        const controller = context.controller;
        enqueue.effect(() => controller.abort());
      }
      return {
        ...context,
        canceled: false,
        controller: event.controller,
        generation:
          context.controller === null || context.controller === event.controller
            ? context.generation
            : context.generation + 1,
        hasLocalRuntimeTail: true,
        pending: true,
      };
    },
  },
});

export function createSessionPromptDispatchStore() {
  return sessionPromptDispatchLogic.createStore();
}

export type SessionPromptDispatchStore = ReturnType<typeof createSessionPromptDispatchStore>;

export const SessionPromptDispatchContext = createContext<SessionPromptDispatchStore | null>(null);
