import {
  MAX_RECONNECT_ATTEMPTS,
  RECONNECT_BASE_DELAY_MS,
  RECONNECT_MAX_DELAY_MS,
  SURFACE_REFRESH_DELAY_MS,
  TRANSCRIPT_RECOVERY_DELAY_MS,
  type SessionLiveTailAppliedFrame,
  type SessionLiveTailContext,
  type SessionLiveTailFrame,
  type SessionLiveTailEnqueue,
  type SessionLiveTailRuntime,
  type SessionLiveTailTrigger,
} from "./session-live-tail-store-contract";

interface SessionLiveTailHandles {
  abortController: AbortController;
  closeStream: ((reason: string) => void) | null;
  queryRecoveryTimer: ReturnType<typeof setTimeout> | null;
  reconnectTimer: ReturnType<typeof setTimeout> | null;
  repairTimer: ReturnType<typeof setTimeout> | null;
  runtime: SessionLiveTailRuntime;
  surfaceRefreshTimer: ReturnType<typeof setTimeout> | null;
}

export const handlesByTrigger = new WeakMap<object, SessionLiveTailHandles>();

export function clearTimer(timer: ReturnType<typeof setTimeout> | null): void {
  if (timer !== null) clearTimeout(timer);
}

export function closeStream(handles: SessionLiveTailHandles, reason: string): void {
  const close = handles.closeStream;
  handles.closeStream = null;
  close?.(reason);
}

export function disposeHandles(handles: SessionLiveTailHandles, reason: string): void {
  clearTimer(handles.queryRecoveryTimer);
  clearTimer(handles.reconnectTimer);
  clearTimer(handles.repairTimer);
  clearTimer(handles.surfaceRefreshTimer);
  handles.abortController.abort();
  closeStream(handles, reason);
}

export function createHandles(runtime: SessionLiveTailRuntime): SessionLiveTailHandles {
  return {
    abortController: new AbortController(),
    closeStream: null,
    queryRecoveryTimer: null,
    reconnectTimer: null,
    repairTimer: null,
    runtime,
    surfaceRefreshTimer: null,
  };
}

export function openStream(
  handles: SessionLiveTailHandles,
  trigger: SessionLiveTailTrigger,
  generation: number
): void {
  try {
    handles.closeStream = handles.runtime.openStream({
      commandsChanged: () => trigger.commandsChanged({ generation }),
      degraded: throughSequence => trigger.degradedReceived({ generation, throughSequence }),
      error: error => trigger.streamError({ at: Date.now(), error, generation }),
      frame: frame => trigger.frameReceived({ at: Date.now(), frame, generation }),
      goalChanged: () => trigger.goalChanged({ generation }),
      terminal: (payload, sequence) => trigger.terminalReceived({ generation, payload, sequence }),
    });
  } catch (error) {
    trigger.streamError({ at: Date.now(), error, generation });
  }
}

/** Upper bound on waiting for a paint: a page hidden before its frame never paints. */
const APPLY_BATCH_FALLBACK_MS = 100;

/**
 * Frames arriving within one paint coalesce into one batch: a visible page
 * applies on the next animation frame, a hidden one on a microtask because
 * nothing paints there.
 */
function scheduleApplyBatch(start: () => void): void {
  if (
    typeof requestAnimationFrame !== "function" ||
    typeof document === "undefined" ||
    document.visibilityState !== "visible"
  ) {
    queueMicrotask(start);
    return;
  }
  let started = false;
  const run = () => {
    if (started) return;
    started = true;
    cancelAnimationFrame(frame);
    clearTimeout(fallback);
    start();
  };
  const frame = requestAnimationFrame(run);
  const fallback = setTimeout(run, APPLY_BATCH_FALLBACK_MS);
}

export function enqueueApplyStart(enqueue: SessionLiveTailEnqueue, generation: number): void {
  enqueue.effect(({ trigger }) => {
    scheduleApplyBatch(() => trigger.applyStarted({ generation }));
  });
}

/**
 * Folds a batch's written frames in order, as each would have completed alone:
 * a reset is stated (US-017.AC-3), and a shed replay ends only when an applied
 * frame reaches its watermark (an empty delta carrying the cursor counts).
 */
export function foldAppliedFrames(
  context: SessionLiveTailContext,
  applied: readonly SessionLiveTailAppliedFrame[],
  at: number
): SessionLiveTailContext {
  let next = context;
  for (const { frame, result } of applied) {
    if (result === "reset") {
      next = {
        ...next,
        historyReset: {
          at,
          generation: frame.payload.generation,
          reason: frame.kind === "snapshot" ? (frame.payload.reason ?? null) : null,
        },
      };
    }
    if (next.catchUpThrough !== null && frame.cursor >= next.catchUpThrough) {
      next = { ...next, catchUpThrough: null, catchingUp: false };
    }
  }
  return next;
}

export function failApply(
  context: SessionLiveTailContext,
  enqueue: SessionLiveTailEnqueue,
  failure: { error: unknown; frame: SessionLiveTailFrame }
): SessionLiveTailContext {
  enqueue.effect(({ trigger }) => {
    const handles = handlesByTrigger.get(trigger);
    if (!handles) return;
    closeStream(handles, "apply-failure");
    handles.runtime.recordApplyFailure(failure.frame, failure.error);
  });
  if (context.pendingTerminal) return refreshBeforePendingTerminal(context, enqueue);
  return scheduleBlockingRepair(context, enqueue);
}

export function enqueueRepairStart(enqueue: SessionLiveTailEnqueue, generation: number): void {
  enqueue.effect(({ trigger }) => {
    queueMicrotask(() => trigger.repairStarted({ generation }));
  });
}

export function scheduleSurfaceRefresh(
  handles: SessionLiveTailHandles,
  trigger: SessionLiveTailTrigger,
  generation: number
): void {
  clearTimer(handles.surfaceRefreshTimer);
  handles.surfaceRefreshTimer = setTimeout(
    () => trigger.surfaceRefreshElapsed({ generation }),
    SURFACE_REFRESH_DELAY_MS
  );
}

export function scheduleQueryRecovery(
  handles: SessionLiveTailHandles,
  trigger: SessionLiveTailTrigger,
  generation: number
): void {
  clearTimer(handles.queryRecoveryTimer);
  handles.queryRecoveryTimer = setTimeout(
    () => trigger.queryRecoveryElapsed({ generation }),
    TRANSCRIPT_RECOVERY_DELAY_MS
  );
}

/**
 * Retry on the backoff curve until the cap; past it the transport gives up
 * visibly (US-018.AC-2): the stream closes, no timer is armed, and only the
 * operator's Try again reopens it. `at` is when this failure was observed.
 */
export function reconnectAfter(
  context: SessionLiveTailContext,
  enqueue: SessionLiveTailEnqueue,
  at: number,
  error?: unknown
): SessionLiveTailContext {
  const attempt = context.reconnectAttempt + 1;
  const degradedAt = context.degradedAt ?? at;
  if (attempt > MAX_RECONNECT_ATTEMPTS) {
    enqueue.effect(({ trigger }) => {
      const handles = handlesByTrigger.get(trigger);
      if (!handles) return;
      clearTimer(handles.reconnectTimer);
      closeStream(handles, "retries-exhausted");
      handles.runtime.recordReconnect(attempt, 0, error);
    });
    return {
      ...context,
      applyPhase: "idle",
      catchUpThrough: null,
      catchingUp: false,
      degradedAt,
      failure: { at, attempts: MAX_RECONNECT_ATTEMPTS },
      overflowed: false,
      pendingFrames: [],
      transportPhase: "failed",
    };
  }
  const delayMs = Math.min(RECONNECT_BASE_DELAY_MS * 2 ** (attempt - 1), RECONNECT_MAX_DELAY_MS);
  enqueue.effect(({ trigger }) => {
    const handles = handlesByTrigger.get(trigger);
    if (!handles) return;
    clearTimer(handles.reconnectTimer);
    closeStream(handles, "reconnect");
    handles.runtime.recordReconnect(attempt, delayMs, error);
    handles.reconnectTimer = setTimeout(
      () => trigger.reconnectElapsed({ generation: context.generation }),
      delayMs
    );
  });
  return {
    ...context,
    applyPhase: "idle",
    catchUpThrough: null,
    catchingUp: false,
    degradedAt,
    overflowed: false,
    pendingFrames: [],
    reconnectAttempt: attempt,
    transportPhase: "waiting-reconnect",
  };
}

export function scheduleBlockingRepair(
  context: SessionLiveTailContext,
  enqueue: SessionLiveTailEnqueue
): SessionLiveTailContext {
  enqueueRepairStart(enqueue, context.generation);
  return {
    ...context,
    applyPhase: "repair-scheduled",
    catchUpThrough: null,
    failure: null,
    overflowed: false,
    pendingFrames: [],
    transportPhase: "connecting",
  };
}

export function closeStreamForTerminal(handles: SessionLiveTailHandles, reason: string): void {
  clearTimer(handles.queryRecoveryTimer);
  clearTimer(handles.reconnectTimer);
  clearTimer(handles.repairTimer);
  clearTimer(handles.surfaceRefreshTimer);
  closeStream(handles, reason);
}

export function completePendingTerminal(
  context: SessionLiveTailContext,
  enqueue: SessionLiveTailEnqueue
): SessionLiveTailContext {
  const terminal = context.pendingTerminal;
  if (!terminal) return context;
  if (context.queryRecoveryPhase === "refreshing") {
    return { ...context, applyPhase: "idle" };
  }

  enqueue.effect(({ trigger }) => {
    const handles = handlesByTrigger.get(trigger);
    if (!handles) return;
    handles.runtime.applyTerminal(terminal.payload, terminal.sequence);
    handles.runtime.invalidateSessionSurfaces();
    disposeHandles(handles, "terminal");
  });
  return {
    ...context,
    applyPhase: "idle",
    catchUpThrough: null,
    catchingUp: false,
    generation: context.generation + 1,
    overflowed: false,
    pendingFrames: [],
    pendingTerminal: null,
    queryRecoveryPhase: "idle",
    surfaceRefreshScheduled: false,
    transportPhase: "terminal",
  };
}

export function refreshBeforePendingTerminal(
  context: SessionLiveTailContext,
  enqueue: SessionLiveTailEnqueue
): SessionLiveTailContext {
  enqueue.effect(async ({ trigger }) => {
    const handles = handlesByTrigger.get(trigger);
    if (!handles) return;
    try {
      await handles.runtime.refreshTranscript(handles.abortController.signal);
    } catch (error) {
      handles.runtime.recordTranscriptFailure(error, { recovery: true });
    }
    trigger.terminalRefreshFinished({ generation: context.generation });
  });
  return {
    ...context,
    applyPhase: "terminal-refreshing",
    catchUpThrough: null,
    catchingUp: false,
    overflowed: false,
    pendingFrames: [],
    queryRecoveryPhase: "idle",
    surfaceRefreshScheduled: false,
    transportPhase: "terminal",
  };
}
