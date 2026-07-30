import { createContext } from "react";
import { createStore } from "@xstate/store";

interface SessionPromptDispatchContext {
  canceled: boolean;
  controller: AbortController | null;
  generation: number;
  pending: boolean;
}

export type SessionPromptDispatchStoreEvents = {
  pendingCanceled: Record<never, never>;
  requestCompleted: { controller: AbortController };
  requestStarted: { controller: AbortController };
};

export function createSessionPromptDispatchStore() {
  return createStore<SessionPromptDispatchContext, SessionPromptDispatchStoreEvents>({
    context: {
      canceled: false,
      controller: null,
      generation: 0,
      pending: false,
    },
    on: {
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
      requestStarted: (context, event) => ({
        ...context,
        canceled: false,
        controller: event.controller,
        pending: true,
      }),
    },
  });
}

export type SessionPromptDispatchStore = ReturnType<typeof createSessionPromptDispatchStore>;

export const SessionPromptDispatchContext = createContext<SessionPromptDispatchStore | null>(null);
