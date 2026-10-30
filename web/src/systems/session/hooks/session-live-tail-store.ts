import { createStoreLogic } from "@xstate/store";

import { MAX_PENDING_FRAMES, transcriptRecoveryDelayMs } from "./session-live-tail-store-contract";
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
  failApply,
  foldAppliedFrames,
  handlesByTrigger,
  openStream,
  reconnectAfter,
  refreshBeforePendingTerminal,
  scheduleBlockingRepair,
  scheduleQueryRecovery,
  scheduleSurfaceRefresh,
} from "./session-live-tail-store-effects";
import { sessionLiveTailRecoveryTransitions } from "./session-live-tail-store-recovery";

export const sessionLiveTailLogic = createStoreLogic<SessionLiveTailContext, SessionLiveTailEvents>(
  {
    context: {
      applyPhase: "idle",
      catchUpThrough: null,
      catchingUp: false,
      degradedAt: null,
      failure: null,
      generation: 0,
      historyReset: null,
      lastLiveAt: null,
      lastTranscriptError: null,
      overflowed: false,
      pendingFrames: [],
      pendingTerminal: null,
      queryRecoveryPhase: "idle",
      reconnectAttempt: 0,
      recoveryAttempt: 0,
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
          catchUpThrough: null,
          catchingUp: false,
          // A fresh connect starts its grace now; a disabled window keeps the last live time.
          degradedAt: event.enabled ? event.at : null,
          failure: null,
          generation,
          historyReset: null,
          overflowed: false,
          pendingFrames: [],
          pendingTerminal: null,
          queryRecoveryPhase:
            event.enabled && context.lastTranscriptError !== null ? "waiting" : "idle",
          reconnectAttempt: 0,
          recoveryAttempt: 0,
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
          catchUpThrough: null,
          catchingUp: false,
          degradedAt: null,
          failure: null,
          generation,
          historyReset: null,
          overflowed: false,
          pendingFrames: [],
          pendingTerminal: null,
          queryRecoveryPhase: "idle",
          reconnectAttempt: 0,
          recoveryAttempt: 0,
          surfaceRefreshScheduled: false,
          transportPhase: "disabled",
        };
      },
      frameReceived: (context, event, enqueue) => {
        if (
          event.generation !== context.generation ||
          context.transportPhase === "disabled" ||
          context.transportPhase === "terminal" ||
          context.transportPhase === "failed" ||
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
              lastLiveAt: event.at,
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
              lastLiveAt: event.at,
              reconnectAttempt: 0,
              surfaceRefreshScheduled: event.frame.kind === "delta",
            },
            enqueue
          );
        }
        const pendingFrames = [...context.pendingFrames, event.frame];
        // The first frame after a reopen ends the degraded phase; a reopen that
        // followed a drop keeps reading "catching up" until its replay drains.
        const liveContext: SessionLiveTailContext = {
          ...context,
          degradedAt: null,
          failure: null,
          lastLiveAt: event.at,
          pendingFrames,
          reconnectAttempt: 0,
          surfaceRefreshScheduled: event.frame.kind === "delta",
          transportPhase: "live",
        };
        if (context.applyPhase === "idle") {
          enqueueApplyStart(enqueue, context.generation);
          return { ...liveContext, applyPhase: "scheduled" };
        }
        return liveContext;
      },
      applyStarted: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "scheduled") return;
        // Every frame queued since the last write applies as one ordered batch.
        const frames = context.pendingFrames;
        if (frames.length === 0) return { ...context, applyPhase: "idle" };
        enqueue.effect(async ({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          try {
            const batch = await handles.runtime.applyFrames(frames, handles.abortController.signal);
            trigger.applyCompleted({ ...batch, at: Date.now(), generation: event.generation });
          } catch (error) {
            trigger.applyFailed({ error, frame: frames[0]!, generation: event.generation });
          }
        });
        return { ...context, applyPhase: "applying", pendingFrames: [] };
      },
      applyCompleted: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "applying") return;
        const applied = foldAppliedFrames(context, event.applied, event.at);
        if (event.stop === "mismatch") {
          return applied.pendingTerminal
            ? refreshBeforePendingTerminal(applied, enqueue)
            : reconnectAfter(applied, enqueue, event.at);
        }
        if (event.stop === "cancelled") {
          return applied.pendingTerminal
            ? completePendingTerminal(applied, enqueue)
            : { ...applied, applyPhase: "idle" };
        }
        if (event.stop !== null) return failApply(applied, enqueue, event.stop);
        if (applied.overflowed) {
          return applied.pendingTerminal
            ? refreshBeforePendingTerminal(applied, enqueue)
            : scheduleBlockingRepair(applied, enqueue);
        }
        if (applied.pendingFrames.length === 0) {
          // A reopen replay drained: the view is at the live edge again.
          const drained =
            applied.catchUpThrough === null ? { ...applied, catchingUp: false } : applied;
          return drained.pendingTerminal
            ? completePendingTerminal(drained, enqueue)
            : { ...drained, applyPhase: "idle" };
        }
        enqueueApplyStart(enqueue, applied.generation);
        return { ...applied, applyPhase: "scheduled" };
      },
      applyFailed: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "applying") return;
        return failApply(context, enqueue, event);
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
          recoveryAttempt: 0,
          transportPhase: "connecting",
        };
      },
      repairFailed: (context, event, enqueue) => {
        if (event.generation !== context.generation || context.applyPhase !== "repairing") return;
        // The first repair ran immediately; its retries start at the base delay.
        const delayMs = transcriptRecoveryDelayMs(context.recoveryAttempt);
        const recoveryAttempt = context.recoveryAttempt + 1;
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          handles.runtime.recordTranscriptFailure(event.error, { recovery: true });
          clearTimer(handles.repairTimer);
          handles.repairTimer = setTimeout(
            () => trigger.repairElapsed({ generation: context.generation }),
            delayMs
          );
        });
        if (context.pendingTerminal) {
          return completePendingTerminal({ ...context, recoveryAttempt }, enqueue);
        }
        return { ...context, applyPhase: "repair-waiting", recoveryAttempt };
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
          context.transportPhase === "failed" ||
          context.applyPhase.startsWith("repair")
        ) {
          return;
        }
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (handles) scheduleSurfaceRefresh(handles, trigger, context.generation);
        });
        return reconnectAfter(
          { ...context, surfaceRefreshScheduled: true },
          enqueue,
          event.at,
          event.error
        );
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
        // Reopened after a drop: the gap replays by sequence before the live edge.
        return { ...context, catchingUp: true, transportPhase: "connecting" };
      },
      degradedReceived: (context, event) => {
        if (
          event.generation !== context.generation ||
          context.transportPhase === "disabled" ||
          context.transportPhase === "terminal" ||
          context.transportPhase === "failed"
        ) {
          return;
        }
        // The server shed this watcher and replays `after..through` on this
        // stream: the view is catching up until the watermark is applied.
        return {
          ...context,
          catchUpThrough: Math.max(context.catchUpThrough ?? 0, event.throughSequence),
          catchingUp: true,
        };
      },
      surfaceRefreshElapsed: (context, event, enqueue) => {
        if (event.generation !== context.generation || !context.surfaceRefreshScheduled) return;
        enqueue.effect(({ trigger }) => {
          handlesByTrigger.get(trigger)?.runtime.invalidateSessionSurfaces("delta");
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
      ...sessionLiveTailRecoveryTransitions,
    },
  }
);
