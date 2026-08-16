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

export const SESSION_DRAFTS_STORAGE_KEY = "compozy:session:drafts:v1";

interface SessionDraftStorageEnvelope {
  drafts: Record<string, string>;
  version: 1;
}

function normalizedDrafts(value: unknown): Record<string, string> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return {};
  }

  return Object.fromEntries(
    Object.entries(value).filter(
      (entry): entry is [string, string] =>
        entry[0].length > 0 && typeof entry[1] === "string" && entry[1].length > 0
    )
  );
}

function draftsFromStorageValue(value: string | null): Record<string, string> {
  if (!value) return {};

  try {
    const parsed: unknown = JSON.parse(value);
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return {};
    const envelope = parsed as Partial<SessionDraftStorageEnvelope>;
    return envelope.version === 1 ? normalizedDrafts(envelope.drafts) : {};
  } catch (error) {
    console.error("Failed to read persisted session drafts", error);
    return {};
  }
}

function readPersistedDrafts(): Record<string, string> {
  if (typeof window === "undefined") return {};
  try {
    return draftsFromStorageValue(window.localStorage.getItem(SESSION_DRAFTS_STORAGE_KEY));
  } catch (error) {
    console.error("Failed to access persisted session drafts", error);
    return {};
  }
}

function writePersistedDrafts(drafts: Record<string, string>): void {
  if (typeof window === "undefined") return;
  try {
    if (Object.keys(drafts).length === 0) {
      window.localStorage.removeItem(SESSION_DRAFTS_STORAGE_KEY);
      return;
    }
    const envelope: SessionDraftStorageEnvelope = { drafts, version: 1 };
    window.localStorage.setItem(SESSION_DRAFTS_STORAGE_KEY, JSON.stringify(envelope));
  } catch (error) {
    console.error("Failed to persist session drafts", error);
  }
}

const initialSessionContext: SessionStoreContext = {
  drafts: readPersistedDrafts(),
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
    persistedDraftsHydrated: (context, event: { drafts: Record<string, string> }) => {
      if (stringRecordsEqual(context.drafts, event.drafts)) return;
      return { ...context, drafts: event.drafts };
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

let observedDrafts = sessionStore.getSnapshot().context.drafts;
sessionStore.subscribe(snapshot => {
  if (snapshot.context.drafts === observedDrafts) return;
  observedDrafts = snapshot.context.drafts;
  writePersistedDrafts(observedDrafts);
});

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === SESSION_DRAFTS_STORAGE_KEY) {
      sessionStore.trigger.persistedDraftsHydrated({
        drafts: draftsFromStorageValue(event.newValue),
      });
    }
  });
}

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

function stringRecordsEqual(left: Record<string, string>, right: Record<string, string>): boolean {
  const leftEntries = Object.entries(left);
  return (
    leftEntries.length === Object.keys(right).length &&
    leftEntries.every(([key, value]) => right[key] === value)
  );
}
