import { createStoreLogic } from "@xstate/store";

import {
  MAX_PENDING_FRAMES,
  TRANSCRIPT_RECOVERY_DELAY_MS,
} from "./session-live-tail-store-contract";
import type {
  SessionLiveTailContext,
  SessionLiveTailEvents,
} from "./session-live-tail-store-contract";
import {
  clearTimer,
  closeStreamForTerminal,
  closeStream,
  completePendingTerminal,
  createHandles,
  disposeHandles,
  enqueueApplyStart,
  handlesByTrigger,
  openStream,
  reconnectAfter,
  refreshBeforePendingTerminal,
  scheduleBlockingRepair,
  scheduleQueryRecovery,
  scheduleSurfaceRefresh,
} from "./session-live-tail-store-effects";

export const sessionLiveTailLogic = createStoreLogic<SessionLiveTailContext, SessionLiveTailEvents>(
  {
    context: {
      applyPhase: "idle",
      generation: 0,
      lastTranscriptError: null,
      overflowed: false,
      pendingFrames: [],
      pendingTerminal: null,
      queryRecoveryPhase: "idle",
      reconnectAttempt: 0,
      surfaceRefreshScheduled: false,
      transportPhase: "disabled",
    },
    on: {
      configured: (context, event, enqueue) => {
        const generation = context.generation + 1;
        enqueue.effect(({ trigger }) => {
          const previous = handlesByTrigger.get(trigger);
          if (previous) disposeHandles(previous, "reconfigure");
          const handles = createHandles(event.runtime);
          handlesByTrigger.set(trigger, handles);
          if (event.enabled) openStream(handles, trigger, generation);
          if (event.enabled && context.lastTranscriptError !== null) {
            scheduleQueryRecovery(handles, trigger, generation);
          }
        });
        return {
          ...context,
          applyPhase: "idle",
          generation,
          overflowed: false,
          pendingFrames: [],
          pendingTerminal: null,
          queryRecoveryPhase:
            event.enabled && context.lastTranscriptError !== null ? "waiting" : "idle",
          reconnectAttempt: 0,
          surfaceRefreshScheduled: false,
          transportPhase: event.enabled ? "connecting" : "disabled",
        };
      },
      disposed: (context, _event, enqueue) => {
        const generation = context.generation + 1;
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (handles) disposeHandles(handles, "cleanup");
          handlesByTrigger.delete(trigger);
        });
        return {
          ...context,
          applyPhase: "idle",
          generation,
          overflowed: false,
          pendingFrames: [],
          pendingTerminal: null,
          queryRecoveryPhase: "idle",
          reconnectAttempt: 0,
          surfaceRefreshScheduled: false,
          transportPhase: "disabled",
        };
      },
      frameReceived: (context, event, enqueue) => {
        if (
          event.generation !== context.generation ||
          context.transportPhase === "disabled" ||
          context.transportPhase === "terminal" ||
          context.applyPhase.startsWith("repair")
        ) {
          return;
        }
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          if (event.frame.kind === "snapshot") {
            clearTimer(handles.surfaceRefreshTimer);
            handles.runtime.invalidateSessionSurfaces();
          } else {
            scheduleSurfaceRefresh(handles, trigger, context.generation);
          }
          if (event.frame.containsClarification) handles.runtime.invalidateClarifications();
        });
        if (context.pendingFrames.length >= MAX_PENDING_FRAMES) {
          enqueue.effect(({ trigger }) => {
            const handles = handlesByTrigger.get(trigger);
            if (handles) closeStream(handles, "backpressure");
          });
          if (context.applyPhase === "applying") {
            return {
              ...context,
              overflowed: true,
              pendingFrames: [],
              reconnectAttempt: 0,
              surfaceRefreshScheduled: event.frame.kind === "delta",
              transportPhase: "connecting",
            };
          }
          return scheduleBlockingRepair(
            {
              ...context,
              reconnectAttempt: 0,
              surfaceRefreshScheduled: event.frame.kind === "delta",
            },
            enqueue
          );
        }
        const pendingFrames = [...context.pendingFrames, event.frame];
        if (context.applyPhase === "idle") {
          enqueueApplyStart(enqueue, context.generation);
          return {
            ...context,
            applyPhase: "scheduled",
            pendingFrames,
            reconnectAttempt: 0,
            surfaceRefreshScheduled: event.frame.kind === "delta",
            transportPhase: "live",
          };
        }
        return {
          ...context,
          pendingFrames,
          reconnectAttempt: 0,
          surfaceRefreshScheduled: event.frame.kind === "delta",
          transportPhase: "live",
        };
      },
      applyStarted: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "scheduled") return;
        const [frame, ...pendingFrames] = context.pendingFrames;
        if (!frame) return { ...context, applyPhase: "idle" };
        enqueue.effect(async ({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          try {
            const result = await handles.runtime.applyFrame(frame, handles.abortController.signal);
            trigger.applyCompleted({ generation: event.generation, result });
          } catch (error) {
            trigger.applyFailed({ error, frame, generation: event.generation });
          }
        });
        return { ...context, applyPhase: "applying", pendingFrames };
      },
      applyCompleted: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "applying") return;
        if (event.result === "mismatch") {
          return context.pendingTerminal
            ? refreshBeforePendingTerminal(context, enqueue)
            : reconnectAfter(context, enqueue);
        }
        if (event.result === "cancelled") {
          return context.pendingTerminal
            ? completePendingTerminal(context, enqueue)
            : { ...context, applyPhase: "idle" };
        }
        if (context.overflowed) {
          return context.pendingTerminal
            ? refreshBeforePendingTerminal(context, enqueue)
            : scheduleBlockingRepair(context, enqueue);
        }
        if (context.pendingFrames.length === 0) {
          return context.pendingTerminal
            ? completePendingTerminal(context, enqueue)
            : { ...context, applyPhase: "idle" };
        }
        enqueueApplyStart(enqueue, context.generation);
        return { ...context, applyPhase: "scheduled" };
      },
      applyFailed: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "applying") return;
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          closeStream(handles, "apply-failure");
          handles.runtime.recordApplyFailure(event.frame, event.error);
        });
        if (context.pendingTerminal) return refreshBeforePendingTerminal(context, enqueue);
        return scheduleBlockingRepair(context, enqueue);
      },
      repairStarted: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "repair-scheduled") {
          return;
        }
        enqueue.effect(async ({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          try {
            await handles.runtime.refreshTranscript(handles.abortController.signal);
            trigger.repairSucceeded({ generation: event.generation });
          } catch (error) {
            trigger.repairFailed({ error, generation: event.generation });
          }
        });
        return { ...context, applyPhase: "repairing" };
      },
      repairSucceeded: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "repairing") return;
        if (context.pendingTerminal) return completePendingTerminal(context, enqueue);
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (handles) openStream(handles, trigger, context.generation);
        });
        return {
          ...context,
          applyPhase: "idle",
          lastTranscriptError: null,
          queryRecoveryPhase: "idle",
          reconnectAttempt: 0,
          transportPhase: "connecting",
        };
      },
      repairFailed: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "repairing") return;
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          handles.runtime.recordTranscriptFailure(event.error, { recovery: true });
          clearTimer(handles.repairTimer);
          handles.repairTimer = setTimeout(
            () => trigger.repairElapsed({ generation: context.generation }),
            TRANSCRIPT_RECOVERY_DELAY_MS
          );
        });
        if (context.pendingTerminal) return completePendingTerminal(context, enqueue);
        return { ...context, applyPhase: "repair-waiting" };
      },
      repairElapsed: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "repair-waiting")
          return;
        return scheduleBlockingRepair(context, enqueue);
      },
      streamError: (context, event, enqueue) => {
        if (
          event.generation !== context.generation ||
          context.transportPhase === "disabled" ||
          context.transportPhase === "terminal" ||
          context.applyPhase.startsWith("repair")
        ) {
          return;
        }
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (handles) scheduleSurfaceRefresh(handles, trigger, context.generation);
        });
        return reconnectAfter({ ...context, surfaceRefreshScheduled: true }, enqueue, event.error);
      },
      reconnectElapsed: (context, event, enqueue) => {
        if (
          event.generation !== context.generation ||
          context.transportPhase !== "waiting-reconnect"
        ) {
          return;
        }
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (handles) openStream(handles, trigger, context.generation);
        });
        return { ...context, transportPhase: "connecting" };
      },
      surfaceRefreshElapsed: (context, event, enqueue) => {
        if (event.generation !== context.generation || !context.surfaceRefreshScheduled) return;
        enqueue.effect(({ trigger }) => {
          handlesByTrigger.get(trigger)?.runtime.invalidateSessionSurfaces();
        });
        return { ...context, surfaceRefreshScheduled: false };
      },
      goalChanged: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.transportPhase === "disabled")
          return;
        enqueue.effect(({ trigger }) => handlesByTrigger.get(trigger)?.runtime.invalidateGoal());
        return { ...context, reconnectAttempt: 0, transportPhase: "live" };
      },
      commandsChanged: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.transportPhase === "disabled")
          return;
        enqueue.effect(({ trigger }) =>
          handlesByTrigger.get(trigger)?.runtime.invalidateCommands()
        );
        return { ...context, reconnectAttempt: 0, transportPhase: "live" };
      },
      terminalReceived: (context, event, enqueue) => {
        if (
          event.generation !== context.generation ||
          context.transportPhase === "disabled" ||
          context.pendingTerminal
        )
          return;
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          closeStreamForTerminal(handles, "terminal-received");
        });
        const terminalContext: SessionLiveTailContext = {
          ...context,
          pendingTerminal: { payload: event.payload, sequence: event.sequence },
          queryRecoveryPhase: context.queryRecoveryPhase === "refreshing" ? "refreshing" : "idle",
          surfaceRefreshScheduled: false,
          transportPhase: "terminal",
        };
        if (context.applyPhase === "idle" && context.pendingFrames.length === 0) {
          return completePendingTerminal(terminalContext, enqueue);
        }
        if (context.applyPhase === "repair-waiting") {
          return refreshBeforePendingTerminal(terminalContext, enqueue);
        }
        return terminalContext;
      },
      terminalRefreshFinished: (context, event, enqueue) => {
        if (
          event.generation !== context.generation ||
          context.applyPhase !== "terminal-refreshing" ||
          !context.pendingTerminal
        ) {
          return;
        }
        return completePendingTerminal(context, enqueue);
      },
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
        if (
          event.generation !== context.generation ||
          context.queryRecoveryPhase !== "refreshing"
        ) {
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
        if (
          event.generation !== context.generation ||
          context.queryRecoveryPhase !== "refreshing"
        ) {
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
    },
  }
);
