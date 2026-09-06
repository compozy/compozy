import { createStoreLogic } from "@xstate/store";

import { SMOOTH_STREAMING_STORAGE_KEY } from "../lib/session-smooth-reveal";

interface SessionStreamingPreferenceContext {
  /** Smooth streaming reveal on assistant prose (ADR-008). Client-local, per browser, on by default. */
  smoothStreaming: boolean;
}

function readSmoothStreaming(): boolean {
  if (typeof window === "undefined") return true;
  try {
    const raw = window.localStorage.getItem(SMOOTH_STREAMING_STORAGE_KEY);
    return raw === null ? true : raw !== "off";
  } catch {
    return true;
  }
}

function writeSmoothStreaming(enabled: boolean): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(SMOOTH_STREAMING_STORAGE_KEY, enabled ? "on" : "off");
  } catch (error) {
    console.warn("Failed to persist the smooth streaming preference", error);
  }
}

export const sessionStreamingPreferenceLogic = createStoreLogic({
  context: (input: { smoothStreaming: boolean }): SessionStreamingPreferenceContext => ({
    smoothStreaming: input.smoothStreaming,
  }),
  on: {
    smoothStreamingSet: (context, event: { enabled: boolean }, enqueue) => {
      if (context.smoothStreaming === event.enabled) return context;
      enqueue.effect(() => writeSmoothStreaming(event.enabled));
      return { ...context, smoothStreaming: event.enabled };
    },
  },
});

export type SessionStreamingPreferenceStore = ReturnType<
  typeof sessionStreamingPreferenceLogic.createStore
>;

/**
 * One process-wide store for the operator's streaming preference (S10 "Smooth
 * streaming"): read once from local storage, written on every change. Never a
 * daemon key by decision (Q20) — it is a presentation preference of this browser.
 */
export const sessionStreamingPreferenceStore: SessionStreamingPreferenceStore =
  sessionStreamingPreferenceLogic.createStore({ smoothStreaming: readSmoothStreaming() });
