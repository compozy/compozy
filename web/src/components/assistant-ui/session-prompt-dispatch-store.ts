import { createContext } from "react";

type Listener = () => void;

export interface SessionPromptDispatchStore {
  cancelPending: () => void;
  complete: (controller: AbortController) => void;
  getGeneration: () => number;
  getSnapshot: () => SessionPromptDispatchSnapshot;
  start: (controller: AbortController) => void;
  subscribe: (listener: Listener) => () => void;
}

export interface SessionPromptDispatchSnapshot {
  canceled: boolean;
  pending: boolean;
}

export function createSessionPromptDispatchStore(): SessionPromptDispatchStore {
  let controller: AbortController | null = null;
  let generation = 0;
  let snapshot: SessionPromptDispatchSnapshot = { canceled: false, pending: false };
  const listeners = new Set<Listener>();

  const emit = () => {
    for (const listener of listeners) listener();
  };

  return {
    cancelPending: () => {
      if (controller === null) return;
      snapshot = { canceled: true, pending: false };
      generation += 1;
      emit();
      controller.abort();
    },
    complete: completedController => {
      if (controller !== completedController) return;
      controller = null;
      if (snapshot.canceled) return;
      snapshot = { canceled: false, pending: false };
      emit();
    },
    getSnapshot: () => snapshot,
    getGeneration: () => generation,
    start: nextController => {
      controller = nextController;
      snapshot = { canceled: false, pending: true };
      emit();
    },
    subscribe: listener => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

export const SessionPromptDispatchContext = createContext(createSessionPromptDispatchStore());
