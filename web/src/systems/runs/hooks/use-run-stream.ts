import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { apiBaseUrl } from "@/lib/api-client";

import {
  defaultRunStreamFactory,
  type RunStreamController,
  type RunStreamFactory,
  type RunStreamSignal,
} from "../lib/stream";

export type RunStreamStatus =
  | "idle"
  | "connecting"
  | "open"
  | "reconnecting"
  | "overflowed"
  | "closed";

export interface RunStreamHeartbeat {
  cursor: string | null;
  receivedAt: number;
}

export interface RunStreamOverflow {
  cursor: string | null;
  reason: string | null;
  receivedAt: number;
}

export interface UseRunStreamOptions {
  runId: string | null;
  enabled?: boolean;
  initialCursor?: string | null;
  reconnectDelayMs?: number;
  maxReconnectAttempts?: number;
  baseUrl?: string;
  factory?: RunStreamFactory;
  onEvent?: (signal: Extract<RunStreamSignal, { type: "event" }>) => void;
  onOverflow?: (overflow: RunStreamOverflow) => void;
  onTerminal?: (error: unknown) => void;
  scheduleReconnect?: (callback: () => void, delayMs: number) => () => void;
}

export interface UseRunStreamResult {
  status: RunStreamStatus;
  lastCursor: string | null;
  lastHeartbeat: RunStreamHeartbeat | null;
  lastOverflow: RunStreamOverflow | null;
  eventCount: number;
  reconnectCount: number;
  error: unknown;
  reconnect: () => void;
  close: () => void;
  resumeFromCursor: (cursor: string | null) => void;
}

const defaultScheduleReconnect = (callback: () => void, delayMs: number): (() => void) => {
  const handle = setTimeout(callback, delayMs);
  return () => clearTimeout(handle);
};

function readOverflowReason(payload: unknown): string | null {
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const reason = Reflect.get(payload, "reason");
  return typeof reason === "string" && reason.length > 0 ? reason : null;
}

export function useRunStream(options: UseRunStreamOptions): UseRunStreamResult {
  const {
    runId,
    enabled = true,
    initialCursor = null,
    reconnectDelayMs = 1_000,
    maxReconnectAttempts = 6,
    factory = defaultRunStreamFactory,
    baseUrl = apiBaseUrl,
    onEvent,
    onOverflow,
    onTerminal,
    scheduleReconnect = defaultScheduleReconnect,
  } = options;

  const [status, setStatus] = useState<RunStreamStatus>("idle");
  const [lastCursor, setLastCursor] = useState<string | null>(initialCursor);
  const [lastHeartbeat, setLastHeartbeat] = useState<RunStreamHeartbeat | null>(null);
  const [lastOverflow, setLastOverflow] = useState<RunStreamOverflow | null>(null);
  const [eventCount, setEventCount] = useState(0);
  const [reconnectCount, setReconnectCount] = useState(0);
  const [error, setError] = useState<unknown>(null);
  const [reconnectKey, setReconnectKey] = useState(0);

  const controllerRef = useRef<RunStreamController | null>(null);
  const cancelReconnectRef = useRef<(() => void) | null>(null);
  const cursorRef = useRef<string | null>(initialCursor);

  const handlersRef = useRef({ onEvent, onOverflow, onTerminal });
  useEffect(() => {
    handlersRef.current = { onEvent, onOverflow, onTerminal };
  }, [onEvent, onOverflow, onTerminal]);

  const reconnect = useCallback(() => {
    setReconnectKey(key => key + 1);
  }, []);

  const close = useCallback(() => {
    controllerRef.current?.close();
    controllerRef.current = null;
    cancelReconnectRef.current?.();
    cancelReconnectRef.current = null;
    setStatus("closed");
  }, []);

  const resumeFromCursor = useCallback(
    (cursor: string | null) => {
      cursorRef.current = cursor;
      setLastCursor(cursor);
      reconnect();
    },
    [reconnect]
  );

  useEffect(() => {
    if (!enabled || !runId) {
      controllerRef.current?.close();
      controllerRef.current = null;
      cancelReconnectRef.current?.();
      cancelReconnectRef.current = null;
      setStatus(enabled ? "idle" : "closed");
      return;
    }

    if (cursorRef.current === null && initialCursor) {
      cursorRef.current = initialCursor;
      setLastCursor(initialCursor);
    }

    setStatus(prev => (prev === "idle" || prev === "closed" ? "connecting" : "reconnecting"));

    const controller = factory({ runId, baseUrl, lastEventId: cursorRef.current }, signal => {
      const snapshot = handlersRef.current;
      switch (signal.type) {
        case "open":
          setStatus("open");
          setError(null);
          return;
        case "event": {
          const cursor = signal.eventId ?? cursorRef.current;
          cursorRef.current = cursor;
          if (cursor !== null) {
            setLastCursor(cursor);
          }
          setEventCount(count => count + 1);
          snapshot.onEvent?.(signal);
          return;
        }
        case "heartbeat": {
          const cursor = signal.eventId ?? cursorRef.current;
          if (cursor !== null) {
            cursorRef.current = cursor;
            setLastCursor(cursor);
          }
          setLastHeartbeat({ cursor, receivedAt: Date.now() });
          return;
        }
        case "overflow": {
          const overflow: RunStreamOverflow = {
            cursor: signal.eventId ?? cursorRef.current,
            reason: readOverflowReason(signal.payload),
            receivedAt: Date.now(),
          };
          setLastOverflow(overflow);
          setStatus("overflowed");
          snapshot.onOverflow?.(overflow);
          return;
        }
        case "error": {
          setError(signal.error);
          snapshot.onTerminal?.(signal.error);
          if (reconnectCount >= maxReconnectAttempts) {
            setStatus("closed");
            return;
          }
          setStatus("reconnecting");
          controllerRef.current?.close();
          controllerRef.current = null;
          cancelReconnectRef.current?.();
          cancelReconnectRef.current = scheduleReconnect(() => {
            cancelReconnectRef.current = null;
            setReconnectCount(count => count + 1);
            setReconnectKey(key => key + 1);
          }, reconnectDelayMs);
          return;
        }
      }
    });
    controllerRef.current = controller;

    return () => {
      controllerRef.current?.close();
      controllerRef.current = null;
      cancelReconnectRef.current?.();
      cancelReconnectRef.current = null;
    };
  }, [
    enabled,
    runId,
    reconnectKey,
    factory,
    baseUrl,
    initialCursor,
    reconnectDelayMs,
    maxReconnectAttempts,
    reconnectCount,
    scheduleReconnect,
  ]);

  return useMemo(
    () => ({
      status,
      lastCursor,
      lastHeartbeat,
      lastOverflow,
      eventCount,
      reconnectCount,
      error,
      reconnect,
      close,
      resumeFromCursor,
    }),
    [
      status,
      lastCursor,
      lastHeartbeat,
      lastOverflow,
      eventCount,
      reconnectCount,
      error,
      reconnect,
      close,
      resumeFromCursor,
    ]
  );
}
