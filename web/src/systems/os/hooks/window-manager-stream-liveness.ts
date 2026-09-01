import type { WindowManagerConnectionStatus } from "../lib/window-manager-types";

/** The daemon writes a heartbeat frame after every protocol ping (30s). */
export const WINDOW_MANAGER_HEARTBEAT_INTERVAL_MS = 30_000;
/** Silence past two heartbeats plus slack means the socket is half-open: drop it and reconnect. */
export const WINDOW_MANAGER_STREAM_STALL_MS = 75_000;
const WINDOW_MANAGER_RECONNECT_BASE_MS = 500;
const WINDOW_MANAGER_RECONNECT_CAP_MS = 8_000;

/** Exponential backoff with up to 25% jitter so many tabs never reconnect in lockstep. */
export function windowManagerReconnectDelay(attempt: number, random = Math.random): number {
  const base = Math.min(
    WINDOW_MANAGER_RECONNECT_CAP_MS,
    WINDOW_MANAGER_RECONNECT_BASE_MS * 2 ** Math.min(attempt, 4)
  );
  return Math.round(base * (1 + random() * 0.25));
}

export interface StreamLivenessInput {
  /** Current reconnect attempt, read when a reconnect is scheduled. */
  reconnectAttempt: () => number;
  /** Fires once the scheduled delay elapses; the caller opens the next socket. */
  onReconnect: (attempt: number) => void;
  onStatus: (status: WindowManagerConnectionStatus) => void;
  /** Detaches handlers and closes the socket this liveness guards. */
  closeSocket: () => void;
}

export interface StreamLiveness {
  /** A frame (or the open event) arrived: the socket is alive right now. */
  noteFrame(): void;
  /** Schedule one reconnect; later triggers for the same socket are ignored. */
  scheduleReconnect(delay: number): void;
  dispose(): void;
}

/**
 * Browsers never surface protocol pings, so a silent socket is only known to
 * be alive through the daemon's heartbeat frames. A laptop that slept or a
 * network that changed leaves the socket looking open long after the daemon
 * gave up on it, so wake and online events verify instead of waiting.
 */
export function createStreamLiveness(input: StreamLivenessInput): StreamLiveness {
  let disposed = false;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let stallTimer: ReturnType<typeof setTimeout> | null = null;
  let lastFrameAt = Date.now();

  const clearStallTimer = () => {
    if (stallTimer === null) return;
    clearTimeout(stallTimer);
    stallTimer = null;
  };

  const scheduleReconnect = (delay: number) => {
    if (disposed || reconnectTimer !== null) return;
    clearStallTimer();
    input.onStatus("reconnecting");
    const attempt = input.reconnectAttempt();
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      input.onReconnect(attempt);
    }, delay);
  };

  const reconnectNow = () => {
    if (disposed) return;
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    input.closeSocket();
    scheduleReconnect(0);
  };

  const armStallTimer = () => {
    clearStallTimer();
    stallTimer = setTimeout(() => {
      stallTimer = null;
      reconnectNow();
    }, WINDOW_MANAGER_STREAM_STALL_MS);
  };

  const verifyOnWake = () => {
    if (disposed) return;
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
      scheduleReconnect(0);
      return;
    }
    if (Date.now() - lastFrameAt > WINDOW_MANAGER_HEARTBEAT_INTERVAL_MS) reconnectNow();
  };
  const onVisibilityChange = () => {
    if (document.visibilityState === "visible") verifyOnWake();
  };
  window.addEventListener("online", verifyOnWake);
  document.addEventListener("visibilitychange", onVisibilityChange);

  return {
    noteFrame() {
      lastFrameAt = Date.now();
      armStallTimer();
    },
    scheduleReconnect,
    dispose() {
      disposed = true;
      window.removeEventListener("online", verifyOnWake);
      document.removeEventListener("visibilitychange", onVisibilityChange);
      clearStallTimer();
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    },
  };
}
