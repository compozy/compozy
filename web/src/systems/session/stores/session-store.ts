import { createStore } from "@xstate/store";

import type { SessionGoalCommandResult } from "../types";

export interface SessionGoalFeedback {
  command?: string;
  errorVisible: boolean;
  result: SessionGoalCommandResult;
}

/**
 * A message typed before its session existed, waiting for the session to accept
 * prompts. `claimed` is what makes delivery exactly-once: the sender flips it
 * before touching the runtime, so a repeated attempt finds nothing to do.
 */
export interface SessionFirstPrompt {
  text: string;
  claimed: boolean;
}

export interface SessionStoreContext {
  drafts: Record<string, string>;
  firstPrompts: Record<string, SessionFirstPrompt>;
  goalFeedback: Record<string, SessionGoalFeedback>;
}

const initialSessionContext: SessionStoreContext = {
  drafts: {},
  firstPrompts: {},
  goalFeedback: {},
};

export const sessionStore = createStore({
  context: initialSessionContext,
  on: {
    allDraftsDiscarded: context => {
      if (Object.keys(context.drafts).length === 0) {
        return;
      }
      return { ...context, drafts: {} };
    },
    composerDraftChanged: (context, event: { sessionId: string; text: string }) => {
      if (!event.text) {
        return discardSessionDraft(context, event.sessionId);
      }
      if (context.drafts[event.sessionId] === event.text) {
        return;
      }
      return { ...context, drafts: { ...context.drafts, [event.sessionId]: event.text } };
    },
    composerDraftDiscarded: (context, event: { sessionId: string }) =>
      discardSessionDraft(context, event.sessionId),
    firstPromptQueued: (context, event: { sessionId: string; text: string }) => {
      if (event.text.trim() === "") {
        return;
      }
      return {
        ...context,
        firstPrompts: {
          ...context.firstPrompts,
          [event.sessionId]: { text: event.text, claimed: false },
        },
      };
    },
    firstPromptClaimed: (context, event: { sessionId: string }) => {
      const prompt = context.firstPrompts[event.sessionId];
      if (!prompt || prompt.claimed) {
        return;
      }
      return {
        ...context,
        firstPrompts: {
          ...context.firstPrompts,
          [event.sessionId]: { ...prompt, claimed: true },
        },
      };
    },
    firstPromptSent: (context, event: { sessionId: string }) =>
      discardFirstPrompt(context, event.sessionId),
    firstPromptRecovered: (context, event: { sessionId: string }) => {
      const prompt = context.firstPrompts[event.sessionId];
      if (!prompt) {
        return;
      }
      // The runtime refused the message, so it becomes an ordinary draft: the
      // operator sees exactly what they typed and decides whether to send it.
      return {
        ...context,
        drafts: { ...context.drafts, [event.sessionId]: prompt.text },
        firstPrompts: withoutSessionValue(context.firstPrompts, event.sessionId),
      };
    },
    goalCommandReported: (
      context,
      event: { command?: string; result: SessionGoalCommandResult; sessionId: string }
    ) => {
      const previous = context.goalFeedback[event.sessionId];
      const command = event.command ?? previous?.command;
      const feedback: SessionGoalFeedback = {
        errorVisible: event.result.outcome === "error",
        result: event.result,
        ...(command === undefined ? {} : { command }),
      };
      return {
        ...context,
        goalFeedback: { ...context.goalFeedback, [event.sessionId]: feedback },
      };
    },
    goalErrorAcknowledged: (context, event: { sessionId: string }) => {
      const feedback = context.goalFeedback[event.sessionId];
      if (!feedback?.errorVisible) {
        return;
      }
      return {
        ...context,
        goalFeedback: {
          ...context.goalFeedback,
          [event.sessionId]: { ...feedback, errorVisible: false },
        },
      };
    },
    sessionInteractionRemoved: (context, event: { sessionId: string }) => {
      const drafts = withoutSessionValue(context.drafts, event.sessionId);
      const firstPrompts = withoutSessionValue(context.firstPrompts, event.sessionId);
      const goalFeedback = withoutSessionValue(context.goalFeedback, event.sessionId);
      if (
        drafts === context.drafts &&
        firstPrompts === context.firstPrompts &&
        goalFeedback === context.goalFeedback
      ) {
        return;
      }
      return { ...context, drafts, firstPrompts, goalFeedback };
    },
  },
});

export type SessionStore = typeof sessionStore;

function discardSessionDraft(
  context: SessionStoreContext,
  sessionId: string
): SessionStoreContext | undefined {
  const drafts = withoutSessionValue(context.drafts, sessionId);
  return drafts === context.drafts ? undefined : { ...context, drafts };
}

function discardFirstPrompt(
  context: SessionStoreContext,
  sessionId: string
): SessionStoreContext | undefined {
  const firstPrompts = withoutSessionValue(context.firstPrompts, sessionId);
  return firstPrompts === context.firstPrompts ? undefined : { ...context, firstPrompts };
}

function withoutSessionValue<T>(values: Record<string, T>, sessionId: string): Record<string, T> {
  if (!(sessionId in values)) {
    return values;
  }
  const { [sessionId]: _removed, ...rest } = values;
  return rest;
}
