import { createStoreLogic } from "@xstate/store";

import { payloadFromProgramFrame } from "../lib/cmd-palette-view-patch";
import type { CmdPaletteViewFrame, CmdPaletteViewPayload } from "../lib/cmd-palette-types";

export const VIEW_BUSY_BUDGET_MS = 150;
export const VIEW_DEGRADED_BUDGET_MS = 3_000;
export const VIEW_CIRCUIT_MISSES = 3;

export type CmdPaletteViewProgramPhase =
  | "opening"
  | "ready"
  | "busy"
  | "degraded"
  | "circuit-open"
  | "unavailable"
  | "closed";

export interface CmdPaletteViewProgramState {
  readonly phase: CmdPaletteViewProgramPhase;
  readonly frame: CmdPaletteViewFrame | null;
  readonly payload: CmdPaletteViewPayload | null;
  readonly pendingSeq: number | null;
  readonly nextSeq: number;
  readonly eventCount: number;
  readonly misses: number;
  readonly error: string | null;
  readonly activeHandlers: readonly string[];
  readonly quarantinedHandlers: Readonly<Record<string, number>>;
  readonly acknowledgedEffects: readonly string[];
  readonly openEpoch: number;
  readonly reloaded: boolean;
  readonly lastEvent: {
    readonly args: readonly unknown[];
    readonly controlled: boolean;
    readonly handler: string;
  } | null;
}

type CmdPaletteViewProgramEvents = {
  openStarted: { preserve: boolean };
  openSucceeded: { frame: CmdPaletteViewFrame };
  openFailed: { error: string };
  eventSent: {
    seq: number;
    controlled: boolean;
    handler: string;
    args: readonly unknown[];
  };
  softBudgetElapsed: { seq: number };
  hardBudgetElapsed: { seq: number };
  frameReceived: { frame: CmdPaletteViewFrame };
  effectsAcknowledged: { ids: readonly string[] };
  retryRequested: Record<never, never>;
  reopenRequested: Record<never, never>;
  reloadRequested: Record<never, never>;
  crashed: { error: string };
  closed: Record<never, never>;
};

export const cmdPaletteViewProgramLogic = createStoreLogic<
  CmdPaletteViewProgramState,
  CmdPaletteViewProgramEvents
>({
  context: initialCmdPaletteViewProgramState(),
  on: {
    openStarted: (state, event) =>
      event.preserve && state.payload
        ? { ...state, phase: "busy", pendingSeq: null, error: null }
        : initialCmdPaletteViewProgramState(),
    openSucceeded: (state, event) => acceptFrame(state, event.frame, true),
    openFailed: (state, event) => ({ ...state, phase: "unavailable", error: event.error }),
    eventSent: (state, event) => {
      if (
        state.phase === "opening" ||
        state.phase === "circuit-open" ||
        state.phase === "unavailable" ||
        state.phase === "closed"
      ) {
        return;
      }
      if (event.seq <= state.nextSeq) return;
      return {
        ...state,
        phase: "ready",
        pendingSeq: event.seq,
        nextSeq: event.seq,
        eventCount: state.eventCount + (event.controlled ? 1 : 0),
        error: null,
        reloaded: false,
        lastEvent: {
          args: [...event.args],
          controlled: event.controlled,
          handler: event.handler,
        },
      };
    },
    softBudgetElapsed: (state, event) => {
      if (state.pendingSeq !== event.seq || state.phase !== "ready") return;
      return { ...state, phase: "busy" };
    },
    hardBudgetElapsed: (state, event) => {
      if (state.pendingSeq !== event.seq || (state.phase !== "ready" && state.phase !== "busy")) {
        return;
      }
      const misses = state.misses + 1;
      return {
        ...state,
        misses,
        phase: misses >= VIEW_CIRCUIT_MISSES ? "circuit-open" : "degraded",
      };
    },
    frameReceived: (state, event) => acceptFrame(state, event.frame, false),
    effectsAcknowledged: (state, event) => ({
      ...state,
      acknowledgedEffects: unique([...state.acknowledgedEffects, ...event.ids]),
    }),
    retryRequested: state => {
      if (state.phase !== "degraded" && state.phase !== "circuit-open") return;
      return { ...state, misses: 0, pendingSeq: null, phase: "ready", error: null };
    },
    reopenRequested: state => ({
      ...state,
      error: null,
      misses: 0,
      openEpoch: state.openEpoch + 1,
      pendingSeq: null,
      phase: state.payload ? "busy" : "opening",
      reloaded: false,
    }),
    reloadRequested: state => {
      if (state.phase === "closed") return;
      return {
        ...state,
        error: null,
        misses: 0,
        openEpoch: state.openEpoch + 1,
        pendingSeq: null,
        phase: state.payload ? "busy" : "opening",
        reloaded: true,
      };
    },
    crashed: (state, event) => ({
      ...state,
      phase: "unavailable",
      pendingSeq: null,
      error: event.error,
    }),
    closed: state => ({ ...state, phase: "closed", pendingSeq: null }),
  },
});

export function initialCmdPaletteViewProgramState(): CmdPaletteViewProgramState {
  return {
    phase: "opening",
    frame: null,
    payload: null,
    pendingSeq: null,
    nextSeq: 0,
    eventCount: 0,
    misses: 0,
    error: null,
    activeHandlers: [],
    quarantinedHandlers: {},
    acknowledgedEffects: [],
    openEpoch: 0,
    reloaded: false,
    lastEvent: null,
  };
}

export function programHandlerIsLive(state: CmdPaletteViewProgramState, handler: string): boolean {
  return state.activeHandlers.includes(handler) || (state.quarantinedHandlers[handler] ?? 0) > 0;
}

function acceptFrame(
  state: CmdPaletteViewProgramState,
  frame: CmdPaletteViewFrame,
  opening: boolean
): CmdPaletteViewProgramState {
  if (!opening && state.frame && frame.revision === state.frame.revision) return state;
  let payload: CmdPaletteViewPayload;
  try {
    payload = payloadFromProgramFrame(state.payload, frame);
  } catch (error) {
    return {
      ...state,
      phase: "unavailable",
      error: error instanceof Error ? error.message : "The view sent an invalid frame.",
    };
  }
  const quarantine = advanceHandlerQuarantine(
    state.activeHandlers,
    state.quarantinedHandlers,
    frame.handlers
  );
  const resolvesPending =
    state.pendingSeq !== null &&
    frame.in_reply_to !== undefined &&
    frame.in_reply_to >= state.pendingSeq;
  return {
    ...state,
    phase: "ready",
    frame,
    payload,
    pendingSeq: resolvesPending || opening ? null : state.pendingSeq,
    misses: resolvesPending || opening ? 0 : state.misses,
    error: null,
    activeHandlers: [...frame.handlers],
    quarantinedHandlers: quarantine,
  };
}

function advanceHandlerQuarantine(
  previous: readonly string[],
  current: Readonly<Record<string, number>>,
  next: readonly string[]
): Readonly<Record<string, number>> {
  const active = new Set(next);
  const quarantine: Record<string, number> = {};
  for (const [handler, frames] of Object.entries(current)) {
    if (!active.has(handler) && frames > 1) quarantine[handler] = frames - 1;
  }
  for (const handler of previous) {
    if (!active.has(handler)) quarantine[handler] = 2;
  }
  return quarantine;
}

function unique(values: readonly string[]): readonly string[] {
  return [...new Set(values.filter(Boolean))];
}
