import { createStoreLogic } from "@xstate/store";

import {
  createCmdPaletteViewBudgetScheduler,
  type CmdPaletteViewBudgetScheduler,
} from "../lib/cmd-palette-view-budget-scheduler";
import {
  createCmdPaletteViewSearchScheduler,
  type CmdPaletteViewSearchScheduler,
} from "../lib/cmd-palette-view-search-scheduler";
import { payloadFromProgramFrame } from "../lib/cmd-palette-view-patch";
import type {
  CmdPaletteViewFrame,
  CmdPaletteViewPayload,
  CmdPaletteViewSessionEvent,
} from "../lib/cmd-palette-types";

type PendingEffectResult = NonNullable<CmdPaletteViewSessionEvent["effect_result"]>;

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
  readonly executedEffects: readonly string[];
  readonly pendingEffectResults: readonly PendingEffectResult[];
  readonly openEpoch: number;
  readonly reloaded: boolean;
  readonly catalogRevision: string;
  readonly searchGeneration: number;
  readonly searchQuery: string;
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
    effectResultConsumed?: boolean;
  };
  softBudgetElapsed: { seq: number };
  hardBudgetElapsed: { seq: number };
  frameReceived: { frame: CmdPaletteViewFrame };
  effectsExecutionStarted: { ids: readonly string[] };
  effectsAcknowledged: {
    ids: readonly string[];
    results: readonly PendingEffectResult[];
  };
  effectResultRestored: { result: PendingEffectResult };
  searchObserved: { handler: string | null; query: string; throttleMs: number };
  searchDispatchReady: { generation: number; handler: string; query: string };
  searchEchoed: { query: string };
  catalogRevisionObserved: { revision: string };
  retryRequested: Record<never, never>;
  reopenRequested: Record<never, never>;
  reloadRequested: Record<never, never>;
  crashed: { error: string };
  closed: Record<never, never>;
};

type CmdPaletteViewProgramEmitted = {
  searchRequested: { handler: string; query: string };
};

export interface CmdPaletteViewProgramLogicOptions {
  readonly budgetScheduler?: CmdPaletteViewBudgetScheduler;
  readonly catalogRevision?: string;
  readonly query?: string;
  readonly searchScheduler?: CmdPaletteViewSearchScheduler;
}

export function createCmdPaletteViewProgramLogic(options: CmdPaletteViewProgramLogicOptions = {}) {
  const budgetScheduler =
    options.budgetScheduler ??
    createCmdPaletteViewBudgetScheduler(VIEW_BUSY_BUDGET_MS, VIEW_DEGRADED_BUDGET_MS);
  const searchScheduler = options.searchScheduler ?? createCmdPaletteViewSearchScheduler();
  return createStoreLogic<
    CmdPaletteViewProgramState,
    CmdPaletteViewProgramEvents,
    CmdPaletteViewProgramEmitted
  >({
    context: initialCmdPaletteViewProgramState(options.query, options.catalogRevision),
    on: {
      openStarted: (state, event, enqueue) => {
        enqueue.effect(budgetScheduler.cancel);
        enqueue.effect(searchScheduler.cancel);
        return event.preserve && state.payload
          ? {
              ...state,
              phase: "busy",
              pendingSeq: null,
              error: null,
              nextSeq: 0,
              eventCount: 0,
              acknowledgedEffects: [],
              executedEffects: [],
              pendingEffectResults: [],
              lastEvent: null,
              activeHandlers: [],
              quarantinedHandlers: {},
            }
          : {
              ...initialCmdPaletteViewProgramState(state.searchQuery, state.catalogRevision),
              openEpoch: state.openEpoch,
            };
      },
      openSucceeded: (state, event, enqueue) => {
        enqueue.effect(budgetScheduler.cancel);
        return acceptFrame(state, event.frame, true);
      },
      openFailed: (state, event, enqueue) => {
        enqueue.effect(budgetScheduler.cancel);
        return { ...state, phase: "unavailable", error: event.error };
      },
      eventSent: (state, event, enqueue) => {
        if (
          state.phase === "opening" ||
          state.phase === "circuit-open" ||
          state.phase === "unavailable" ||
          state.phase === "closed"
        ) {
          return;
        }
        if (event.seq <= state.nextSeq) return;
        enqueue.effect(({ trigger }) => {
          budgetScheduler.schedule({
            onSoftBudget: () => trigger.softBudgetElapsed({ seq: event.seq }),
            onHardBudget: () => trigger.hardBudgetElapsed({ seq: event.seq }),
          });
        });
        return {
          ...state,
          phase: "ready",
          pendingSeq: event.seq,
          nextSeq: event.seq,
          eventCount: state.eventCount + (event.controlled ? 1 : 0),
          pendingEffectResults: event.effectResultConsumed
            ? state.pendingEffectResults.slice(1)
            : state.pendingEffectResults,
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
      frameReceived: (state, event, enqueue) => {
        const next = acceptFrame(state, event.frame, false);
        if (next.pendingSeq === null) enqueue.effect(budgetScheduler.cancel);
        return next;
      },
      effectsExecutionStarted: (state, event) => ({
        ...state,
        executedEffects: unique([...state.executedEffects, ...event.ids]),
      }),
      effectsAcknowledged: (state, event) => ({
        ...state,
        acknowledgedEffects: unique([...state.acknowledgedEffects, ...event.ids]),
        pendingEffectResults: [...state.pendingEffectResults, ...event.results],
      }),
      effectResultRestored: (state, event) => ({
        ...state,
        pendingEffectResults: [event.result, ...state.pendingEffectResults],
      }),
      searchObserved: (state, event, enqueue) => {
        if (event.query === state.searchQuery) return;
        const generation = state.searchGeneration + 1;
        if (event.handler === null) {
          enqueue.effect(searchScheduler.cancel);
        } else {
          const handler = event.handler;
          enqueue.effect(({ trigger }) => {
            searchScheduler.schedule(event.throttleMs, () =>
              trigger.searchDispatchReady({ generation, handler, query: event.query })
            );
          });
        }
        return { ...state, searchGeneration: generation, searchQuery: event.query };
      },
      searchDispatchReady: (state, event, enqueue) => {
        if (event.generation !== state.searchGeneration || event.query !== state.searchQuery)
          return;
        enqueue.emit.searchRequested({ handler: event.handler, query: event.query });
        return state;
      },
      searchEchoed: (state, event, enqueue) => {
        if (event.query === state.searchQuery) return;
        enqueue.effect(searchScheduler.cancel);
        return {
          ...state,
          searchGeneration: state.searchGeneration + 1,
          searchQuery: event.query,
        };
      },
      catalogRevisionObserved: (state, event, enqueue) => {
        if (event.revision === state.catalogRevision) return;
        enqueue.trigger.reloadRequested({});
        return { ...state, catalogRevision: event.revision };
      },
      retryRequested: (state, _event, enqueue) => {
        if (state.phase !== "degraded" && state.phase !== "circuit-open") return;
        enqueue.effect(budgetScheduler.cancel);
        return { ...state, misses: 0, pendingSeq: null, phase: "ready", error: null };
      },
      reopenRequested: (state, _event, enqueue) => {
        enqueue.effect(budgetScheduler.cancel);
        enqueue.effect(searchScheduler.cancel);
        return {
          ...state,
          error: null,
          misses: 0,
          openEpoch: state.openEpoch + 1,
          pendingSeq: null,
          executedEffects: [],
          pendingEffectResults: [],
          phase: state.payload ? "busy" : "opening",
          reloaded: false,
        };
      },
      reloadRequested: (state, _event, enqueue) => {
        if (state.phase === "closed") return;
        enqueue.effect(budgetScheduler.cancel);
        enqueue.effect(searchScheduler.cancel);
        return {
          ...state,
          error: null,
          misses: 0,
          openEpoch: state.openEpoch + 1,
          pendingSeq: null,
          executedEffects: [],
          pendingEffectResults: [],
          phase: state.payload ? "busy" : "opening",
          reloaded: true,
        };
      },
      crashed: (state, event, enqueue) => {
        enqueue.effect(budgetScheduler.cancel);
        enqueue.effect(searchScheduler.cancel);
        return {
          ...state,
          phase: "unavailable",
          pendingSeq: null,
          error: event.error,
        };
      },
      closed: (state, _event, enqueue) => {
        enqueue.effect(budgetScheduler.cancel);
        enqueue.effect(searchScheduler.cancel);
        return { ...state, phase: "closed", pendingSeq: null };
      },
    },
  });
}

export function initialCmdPaletteViewProgramState(
  searchQuery = "",
  catalogRevision = ""
): CmdPaletteViewProgramState {
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
    executedEffects: [],
    pendingEffectResults: [],
    openEpoch: 0,
    reloaded: false,
    catalogRevision,
    searchGeneration: 0,
    searchQuery,
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
  if (!opening && state.frame) {
    if (frame.view_session !== state.frame.view_session) return state;
    if (frame.generation < state.frame.generation) return state;
    if (frame.revision === state.frame.revision) return state;
    if (frame.patch && frame.patch.from !== state.frame.revision) {
      return {
        ...state,
        phase: "unavailable",
        error: "The view sent a stale patch.",
      };
    }
  }
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
  return {
    ...state,
    phase: "ready",
    frame,
    payload,
    pendingSeq: null,
    misses: 0,
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
