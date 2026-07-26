import type { StateCreator } from "zustand";
import type { SessionGoalCommandResult } from "../types";

export interface ComposerDraft {
  text: string;
  channel?: string;
}

export interface SessionState {
  drafts: Record<string, ComposerDraft>;
  goalResults: Record<string, SessionGoalCommandResult>;
  goalResultCommands: Record<string, string>;
  goalErrorVisible: Record<string, boolean>;
}

export interface SessionActions {
  setDraft: (sessionId: string, patch: Partial<ComposerDraft>) => void;
  clearDraft: (sessionId: string) => void;
  clearAllDrafts: () => void;
  setGoalResult: (sessionId: string, result: SessionGoalCommandResult, command?: string) => void;
  dismissGoalError: (sessionId: string) => void;
}

export type SessionStore = SessionState & SessionActions;

export const initialSessionState: SessionState = {
  drafts: {},
  goalResults: {},
  goalResultCommands: {},
  goalErrorVisible: {},
};

export const createSessionStore: StateCreator<SessionStore> = set => ({
  ...initialSessionState,

  setDraft: (sessionId, patch) =>
    set(state => {
      const current = state.drafts[sessionId] ?? { text: "" };
      const next: ComposerDraft = { ...current, ...patch };
      const isEmpty = !next.text && !next.channel;
      if (isEmpty) {
        if (!(sessionId in state.drafts)) {
          return state;
        }
        const { [sessionId]: _removed, ...rest } = state.drafts;
        return { drafts: rest };
      }
      return { drafts: { ...state.drafts, [sessionId]: next } };
    }),

  clearDraft: sessionId =>
    set(state => {
      if (!(sessionId in state.drafts)) {
        return state;
      }
      const { [sessionId]: _removed, ...rest } = state.drafts;
      return { drafts: rest };
    }),

  clearAllDrafts: () => set({ drafts: {} }),

  setGoalResult: (sessionId, result, command) =>
    set(state => ({
      goalResults: { ...state.goalResults, [sessionId]: result },
      goalResultCommands: command
        ? { ...state.goalResultCommands, [sessionId]: command }
        : state.goalResultCommands,
      goalErrorVisible:
        result.outcome === "error"
          ? { ...state.goalErrorVisible, [sessionId]: true }
          : withoutSessionKey(state.goalErrorVisible, sessionId),
    })),

  dismissGoalError: sessionId =>
    set(state => ({ goalErrorVisible: withoutSessionKey(state.goalErrorVisible, sessionId) })),
});

function withoutSessionKey<T>(values: Record<string, T>, sessionId: string): Record<string, T> {
  if (!(sessionId in values)) {
    return values;
  }
  const { [sessionId]: _removed, ...rest } = values;
  return rest;
}
