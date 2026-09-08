import type {
  SessionLiveTailContext,
  SessionLiveTailEnqueue,
  SessionLiveTailEvents,
} from "./session-live-tail-store-contract";
import {
  clearTimer,
  completePendingTerminal,
  handlesByTrigger,
  scheduleBlockingRepair,
  scheduleQueryRecovery,
} from "./session-live-tail-store-effects";

type Transition<K extends keyof SessionLiveTailEvents> = (
  context: SessionLiveTailContext,
  event: SessionLiveTailEvents[K] & { type: K },
  enqueue: SessionLiveTailEnqueue
) => SessionLiveTailContext | undefined;

type RecoveryTransitions = {
  [K in
    | "transcriptObserved"
    | "queryRecoveryElapsed"
    | "queryRecoverySucceeded"
    | "queryRecoveryFailed"
    | "manualRecoveryRequested"]: Transition<K>;
};

/**
 * The transcript-query recovery half of the live tail: a failed transcript
 * read retries on a bounded cadence while the stream is alive, and the
 * operator's Try again either rereads (query failure) or, once the transport
 * gave up, rereads and reopens the stream with a fresh attempt budget.
 */
export const sessionLiveTailRecoveryTransitions: RecoveryTransitions = {
  transcriptObserved: (context, event, enqueue) => {
    const recoveryActive = context.queryRecoveryPhase === "refreshing";
    if (event.error === null) {
      enqueue.effect(({ trigger }) => {
        const handles = handlesByTrigger.get(trigger);
        if (!handles) return;
        clearTimer(handles.queryRecoveryTimer);
        handles.queryRecoveryTimer = null;
      });
      return {
        ...context,
        lastTranscriptError: null,
        queryRecoveryPhase: recoveryActive ? "refreshing" : "idle",
      };
    }
    if (Object.is(event.error, context.lastTranscriptError)) return;
    enqueue.effect(({ trigger }) => {
      const handles = handlesByTrigger.get(trigger);
      if (!handles) return;
      handles.runtime.recordTranscriptFailure(event.error, {
        recovery: false,
        sessionState: event.sessionState,
      });
      if (
        !recoveryActive &&
        context.transportPhase !== "disabled" &&
        context.transportPhase !== "terminal"
      ) {
        scheduleQueryRecovery(handles, trigger, context.generation);
      }
    });
    return {
      ...context,
      lastTranscriptError: event.error,
      queryRecoveryPhase: recoveryActive
        ? "refreshing"
        : context.transportPhase === "disabled" || context.transportPhase === "terminal"
          ? "idle"
          : "waiting",
    };
  },
  queryRecoveryElapsed: (context, event, enqueue) => {
    if (
      event.generation !== context.generation ||
      context.queryRecoveryPhase !== "waiting" ||
      context.transportPhase === "disabled" ||
      context.transportPhase === "terminal"
    ) {
      return;
    }
    enqueue.effect(async ({ trigger }) => {
      const handles = handlesByTrigger.get(trigger);
      if (!handles) return;
      try {
        await handles.runtime.refreshTranscript(handles.abortController.signal);
        trigger.queryRecoverySucceeded({ generation: event.generation });
      } catch (error) {
        trigger.queryRecoveryFailed({ error, generation: event.generation });
      }
    });
    return { ...context, queryRecoveryPhase: "refreshing" };
  },
  queryRecoverySucceeded: (context, event, enqueue) => {
    if (event.generation !== context.generation || context.queryRecoveryPhase !== "refreshing") {
      return;
    }
    const recoveredContext = {
      ...context,
      lastTranscriptError: null,
      queryRecoveryPhase: "idle" as const,
    };
    if (
      context.pendingTerminal &&
      context.applyPhase === "idle" &&
      context.pendingFrames.length === 0
    ) {
      return completePendingTerminal(recoveredContext, enqueue);
    }
    return recoveredContext;
  },
  queryRecoveryFailed: (context, event, enqueue) => {
    if (event.generation !== context.generation || context.queryRecoveryPhase !== "refreshing") {
      return;
    }
    enqueue.effect(({ trigger }) => {
      const handles = handlesByTrigger.get(trigger);
      if (!handles) return;
      handles.runtime.recordTranscriptFailure(event.error, { recovery: true });
      if (context.transportPhase !== "disabled" && context.transportPhase !== "terminal") {
        scheduleQueryRecovery(handles, trigger, context.generation);
      }
    });
    const recoveredContext = { ...context, queryRecoveryPhase: "idle" as const };
    if (
      context.pendingTerminal &&
      context.applyPhase === "idle" &&
      context.pendingFrames.length === 0
    ) {
      return completePendingTerminal(recoveredContext, enqueue);
    }
    return {
      ...recoveredContext,
      queryRecoveryPhase:
        context.transportPhase === "disabled" || context.transportPhase === "terminal"
          ? "idle"
          : "waiting",
    };
  },
  manualRecoveryRequested: (context, _event, enqueue) => {
    if (context.queryRecoveryPhase === "refreshing") return;
    if (context.transportPhase === "failed" && context.applyPhase === "idle") {
      // Try again after exhausted retries: reread the transcript, then reopen
      // the stream from the durable cursor with a fresh attempt budget.
      return scheduleBlockingRepair({ ...context, catchingUp: true, reconnectAttempt: 0 }, enqueue);
    }
    enqueue.effect(async ({ trigger }) => {
      const handles = handlesByTrigger.get(trigger);
      if (!handles) return;
      clearTimer(handles.queryRecoveryTimer);
      try {
        await handles.runtime.refreshTranscript(handles.abortController.signal);
        trigger.queryRecoverySucceeded({ generation: context.generation });
      } catch (error) {
        trigger.queryRecoveryFailed({ error, generation: context.generation });
      }
    });
    return { ...context, queryRecoveryPhase: "refreshing" };
  },
};
