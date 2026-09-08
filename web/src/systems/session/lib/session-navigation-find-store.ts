import { createStoreLogic } from "@xstate/store";

import { reconcileFindMatches, stepFindMatch } from "./session-navigation";
import type { SessionTranscriptSearchMatch } from "../types";

/*
 * Find's interaction state (S8): which match is active, whether a jump is
 * loading older history or landing, and how many matches streaming appended
 * since the reader last looked. Server results stay in the query cache; this
 * store only holds what the reader did with them.
 */

export interface SessionFindJump {
  sequence: number;
  /** `loading` while older pages come in for an unloaded target; `landing` while the host scrolls/opens. */
  phase: "loading" | "landing";
}

export interface SessionFindContext {
  activeSequence: number | null;
  /** Matches appended by streaming since the last step/selection (US-028.EC-2). */
  appended: number;
  jump: SessionFindJump | null;
  jumpError: unknown | null;
  /** The last list observed, so a refresh can be reconciled by identity. */
  observed: readonly SessionTranscriptSearchMatch[];
}

/** How the host lands a jump: load history until the entry is present, then open the fold and scroll. */
export interface SessionFindJumpHandlers {
  isSequenceLoaded: (sequence: number) => boolean;
  /** Resolves `true` once the entry starting at `sequence` is loaded; `false` when history ran out first. */
  loadOlderUntil: (sequence: number) => Promise<boolean>;
  /** Opens the fold around the entry and scrolls it into view under the bar (free mode). */
  jumpToSequence: (sequence: number) => Promise<void> | void;
}

export type SessionFindEvents = {
  appendedAcknowledged: Record<never, never>;
  closed: Record<never, never>;
  jumpFailed: { error: unknown; sequence: number };
  jumpLanding: { sequence: number };
  jumpRequested: { handlers: SessionFindJumpHandlers; sequence: number };
  jumpSettled: { sequence: number };
  matchesObserved: { matches: readonly SessionTranscriptSearchMatch[] };
  queryChanged: Record<never, never>;
  /** ↑/↓ in the list: moves the selection without jumping. */
  selected: { sequence: number };
  /** Enter / Shift+Enter / the step buttons: moves and jumps. */
  stepped: { direction: 1 | -1; handlers: SessionFindJumpHandlers };
};

const IDLE: SessionFindContext = {
  activeSequence: null,
  appended: 0,
  jump: null,
  jumpError: null,
  observed: [],
};

function beginJump(
  context: SessionFindContext,
  sequence: number,
  handlers: SessionFindJumpHandlers,
  enqueue: { effect: (fn: (api: { trigger: SessionFindTrigger }) => void | Promise<void>) => void }
): SessionFindContext {
  const loaded = handlers.isSequenceLoaded(sequence);
  enqueue.effect(async ({ trigger }) => {
    try {
      if (!loaded) {
        const reached = await handlers.loadOlderUntil(sequence);
        if (!reached) {
          trigger.jumpFailed({
            error: new Error("That part of the history is not loaded."),
            sequence,
          });
          return;
        }
        trigger.jumpLanding({ sequence });
      }
      await handlers.jumpToSequence(sequence);
      trigger.jumpSettled({ sequence });
    } catch (error) {
      trigger.jumpFailed({ error, sequence });
    }
  });
  return {
    ...context,
    activeSequence: sequence,
    appended: 0,
    jump: { phase: loaded ? "landing" : "loading", sequence },
    jumpError: null,
  };
}

export interface SessionFindTrigger {
  jumpFailed: (event: SessionFindEvents["jumpFailed"]) => void;
  jumpLanding: (event: SessionFindEvents["jumpLanding"]) => void;
  jumpSettled: (event: SessionFindEvents["jumpSettled"]) => void;
}

export const sessionFindLogic = createStoreLogic<SessionFindContext, SessionFindEvents>({
  context: IDLE,
  on: {
    appendedAcknowledged: context =>
      context.appended === 0 ? undefined : { ...context, appended: 0 },
    closed: () => IDLE,
    queryChanged: context => ({
      ...context,
      activeSequence: null,
      appended: 0,
      jump: null,
      jumpError: null,
    }),
    matchesObserved: (context, event) => {
      const { activeSequence, appended } = reconcileFindMatches(
        context.observed,
        event.matches,
        context.activeSequence
      );
      return {
        ...context,
        activeSequence,
        appended: context.appended + appended,
        observed: event.matches,
      };
    },
    selected: (context, event) => ({ ...context, activeSequence: event.sequence, appended: 0 }),
    stepped: (context, event, enqueue) => {
      // Steps are blocked while a jump is still loading history (never jump on a stale set).
      if (context.jump?.phase === "loading") return;
      const next = stepFindMatch(context.observed, context.activeSequence, event.direction);
      if (next === null) return;
      return beginJump(context, next, event.handlers, enqueue);
    },
    jumpRequested: (context, event, enqueue) => {
      if (context.jump?.phase === "loading") return;
      return beginJump(context, event.sequence, event.handlers, enqueue);
    },
    jumpLanding: (context, event) =>
      context.jump?.sequence === event.sequence
        ? { ...context, jump: { phase: "landing", sequence: event.sequence } }
        : undefined,
    jumpSettled: (context, event) =>
      context.jump?.sequence === event.sequence ? { ...context, jump: null } : undefined,
    jumpFailed: (context, event) =>
      context.jump?.sequence === event.sequence
        ? { ...context, jump: null, jumpError: event.error }
        : undefined,
  },
});
