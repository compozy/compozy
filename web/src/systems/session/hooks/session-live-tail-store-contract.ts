import type { EnqueueObject } from "@xstate/store";

import type {
  SessionEventPayload,
  TranscriptDeltaPayload,
  TranscriptSnapshotPayload,
} from "../types";

export const MAX_PENDING_FRAMES = 64;
export const RECONNECT_BASE_DELAY_MS = 250;
export const RECONNECT_MAX_DELAY_MS = 4_000;
export const SURFACE_REFRESH_DELAY_MS = 120;
export const TRANSCRIPT_RECOVERY_DELAY_MS = 5_000;

export type SessionLiveTailFrame =
  | {
      containsClarification: boolean;
      cursor: number;
      kind: "delta";
      payload: TranscriptDeltaPayload;
    }
  | {
      containsClarification: boolean;
      cursor: number;
      kind: "snapshot";
      payload: TranscriptSnapshotPayload;
    };

export type SessionLiveTailApplyResult = "applied" | "cancelled" | "mismatch";

export interface SessionLiveTailStreamHandlers {
  commandsChanged: () => void;
  error: (error?: unknown) => void;
  frame: (frame: SessionLiveTailFrame) => void;
  goalChanged: () => void;
  terminal: (payload: SessionEventPayload | null, sequence: number) => void;
}

export interface SessionLiveTailRuntime {
  applyFrame: (
    frame: SessionLiveTailFrame,
    signal: AbortSignal
  ) => Promise<SessionLiveTailApplyResult>;
  applyTerminal: (payload: SessionEventPayload | null, sequence: number) => void;
  invalidateClarifications: () => void;
  invalidateCommands: () => void;
  invalidateGoal: () => void;
  invalidateSessionSurfaces: () => void;
  openStream: (handlers: SessionLiveTailStreamHandlers) => (reason: string) => void;
  recordApplyFailure: (frame: SessionLiveTailFrame, error: unknown) => void;
  recordReconnect: (attempt: number, delayMs: number, error?: unknown) => void;
  recordTranscriptFailure: (
    error: unknown,
    options: { recovery: boolean; sessionState?: string }
  ) => void;
  refreshTranscript: (signal: AbortSignal) => Promise<void>;
  refreshTerminalTranscript: () => Promise<void>;
}

export type ApplyPhase =
  | "idle"
  | "scheduled"
  | "applying"
  | "repair-scheduled"
  | "repairing"
  | "repair-waiting";
export type QueryRecoveryPhase = "idle" | "waiting" | "refreshing";
export type TransportPhase = "disabled" | "connecting" | "live" | "waiting-reconnect" | "terminal";

export interface SessionLiveTailContext {
  applyPhase: ApplyPhase;
  generation: number;
  lastTranscriptError: unknown | null;
  overflowed: boolean;
  pendingFrames: readonly SessionLiveTailFrame[];
  queryRecoveryPhase: QueryRecoveryPhase;
  reconnectAttempt: number;
  surfaceRefreshScheduled: boolean;
  transportPhase: TransportPhase;
}

export type SessionLiveTailEvents = {
  applyCompleted: { generation: number; result: SessionLiveTailApplyResult };
  applyFailed: { error: unknown; frame: SessionLiveTailFrame; generation: number };
  applyStarted: { generation: number };
  commandsChanged: { generation: number };
  configured: { enabled: boolean; runtime: SessionLiveTailRuntime };
  disposed: Record<never, never>;
  frameReceived: { frame: SessionLiveTailFrame; generation: number };
  goalChanged: { generation: number };
  manualRecoveryRequested: Record<never, never>;
  queryRecoveryElapsed: { generation: number };
  queryRecoveryFailed: { error: unknown; generation: number };
  queryRecoverySucceeded: { generation: number };
  reconnectElapsed: { generation: number };
  repairElapsed: { generation: number };
  repairFailed: { error: unknown; generation: number };
  repairStarted: { generation: number };
  repairSucceeded: { generation: number };
  streamError: { error?: unknown; generation: number };
  surfaceRefreshElapsed: { generation: number };
  terminalReceived: {
    generation: number;
    payload: SessionEventPayload | null;
    sequence: number;
  };
  transcriptObserved: { error: unknown | null; sessionState?: string };
};

export type SessionLiveTailEnqueue = EnqueueObject<
  SessionLiveTailContext,
  never,
  SessionLiveTailEvents
>;

export interface SessionLiveTailTrigger {
  applyCompleted: (event: SessionLiveTailEvents["applyCompleted"]) => void;
  applyFailed: (event: SessionLiveTailEvents["applyFailed"]) => void;
  applyStarted: (event: SessionLiveTailEvents["applyStarted"]) => void;
  commandsChanged: (event: SessionLiveTailEvents["commandsChanged"]) => void;
  frameReceived: (event: SessionLiveTailEvents["frameReceived"]) => void;
  goalChanged: (event: SessionLiveTailEvents["goalChanged"]) => void;
  queryRecoveryElapsed: (event: SessionLiveTailEvents["queryRecoveryElapsed"]) => void;
  queryRecoveryFailed: (event: SessionLiveTailEvents["queryRecoveryFailed"]) => void;
  queryRecoverySucceeded: (event: SessionLiveTailEvents["queryRecoverySucceeded"]) => void;
  reconnectElapsed: (event: SessionLiveTailEvents["reconnectElapsed"]) => void;
  repairElapsed: (event: SessionLiveTailEvents["repairElapsed"]) => void;
  repairFailed: (event: SessionLiveTailEvents["repairFailed"]) => void;
  repairStarted: (event: SessionLiveTailEvents["repairStarted"]) => void;
  repairSucceeded: (event: SessionLiveTailEvents["repairSucceeded"]) => void;
  streamError: (event: SessionLiveTailEvents["streamError"]) => void;
  surfaceRefreshElapsed: (event: SessionLiveTailEvents["surfaceRefreshElapsed"]) => void;
  terminalReceived: (event: SessionLiveTailEvents["terminalReceived"]) => void;
}
