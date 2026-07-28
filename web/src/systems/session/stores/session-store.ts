import { createStore } from "@xstate/store";

import type { SessionGoalCommandResult } from "../types";

export interface SessionGoalFeedback {
  command?: string;
  errorVisible: boolean;
  result: SessionGoalCommandResult;
}

export interface SessionStoreContext {
  drafts: Record<string, string>;
  goalFeedback: Record<string, SessionGoalFeedback>;
}

const initialSessionContext: SessionStoreContext = {
  drafts: {},
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
      const goalFeedback = withoutSessionValue(context.goalFeedback, event.sessionId);
      if (drafts === context.drafts && goalFeedback === context.goalFeedback) {
        return;
      }
      return { ...context, drafts, goalFeedback };
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

function withoutSessionValue<T>(values: Record<string, T>, sessionId: string): Record<string, T> {
  if (!(sessionId in values)) {
    return values;
  }
  const { [sessionId]: _removed, ...rest } = values;
  return rest;
}
