import { createContext } from "react";
import { createStoreLogic } from "@xstate/store";

import type { SessionSendEnvelope } from "@/systems/session/lib/session-unconfirmed-send";

interface SessionPromptDispatchContext {
  canceled: boolean;
  controller: AbortController | null;
  hasLocalRuntimeTail: boolean;
  pending: boolean;
  /** When the pending request left (clock passed in by the effect); the thinking guard counts from here. */
  pendingSinceMs: number | null;
  /**
   * The prompt on the wire and whether the daemon answered it at all. A
   * response of any status is an authoritative outcome (accepted and
   * streaming, or refused); only a request that never got one leaves
   * admission unknown, so its identity stays retryable.
   */
  prompt: { answeredStatus: number | null; envelope: SessionSendEnvelope } | null;
  streamResetGeneration: number;
}

export type SessionPromptDispatchStoreEvents = {
  conversationReset: Record<never, never>;
  pendingCanceled: Record<never, never>;
  /** The daemon replied to the prompt POST with this HTTP status. */
  promptAnswered: { status: number };
  /** The transport failed; `unconfirmed` says the request got no answer and its identity is retained. */
  promptFailed: { unconfirmed: boolean };
  /** The prompt body left for the wire with this identity and content. */
  promptPrepared: { envelope: SessionSendEnvelope };
  requestCompleted: { controller: AbortController };
  requestStarted: { controller: AbortController; at?: number };
};

export type SessionPromptDispatchEmittedEvents = {
  sendUnconfirmed: { envelope: SessionSendEnvelope };
};

export const sessionPromptDispatchLogic = createStoreLogic<
  SessionPromptDispatchContext,
  SessionPromptDispatchStoreEvents,
  SessionPromptDispatchEmittedEvents
>({
  context: {
    canceled: false,
    controller: null,
    hasLocalRuntimeTail: false,
    pending: false,
    pendingSinceMs: null,
    prompt: null,
    streamResetGeneration: 0,
  },
  on: {
    conversationReset: context => ({
      ...context,
      hasLocalRuntimeTail: false,
      streamResetGeneration: context.streamResetGeneration + 1,
    }),
    promptAnswered: (context, event) =>
      context.prompt === null
        ? undefined
        : { ...context, prompt: { ...context.prompt, answeredStatus: event.status } },
    promptFailed: (context, event, enqueue) => {
      if (context.prompt !== null && event.unconfirmed) {
        enqueue.emit.sendUnconfirmed({ envelope: context.prompt.envelope });
      }
      return { ...context, prompt: null };
    },
    promptPrepared: (context, event) => ({
      ...context,
      prompt: { answeredStatus: null, envelope: event.envelope },
    }),
    pendingCanceled: (context, _event, enqueue) => {
      if (context.controller === null) return;
      const controller = context.controller;
      enqueue.effect(() => controller.abort());
      return {
        ...context,
        canceled: true,
        controller: null,
        pending: false,
        pendingSinceMs: null,
      };
    },
    requestCompleted: (context, event) => {
      if (context.controller !== event.controller) return;
      return {
        ...context,
        canceled: false,
        controller: null,
        pending: false,
        pendingSinceMs: null,
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
        hasLocalRuntimeTail: true,
        pending: true,
        pendingSinceMs: event.at ?? null,
      };
    },
  },
});

export function createSessionPromptDispatchStore() {
  return sessionPromptDispatchLogic.createStore();
}

export type SessionPromptDispatchStore = ReturnType<typeof createSessionPromptDispatchStore>;

export const SessionPromptDispatchContext = createContext<SessionPromptDispatchStore | null>(null);
