import { createStoreLogic } from "@xstate/store";
import type { Rnd } from "react-rnd";

import type { OsRect } from "../lib/os-types";

interface OsWindowInteractionState {
  gestureAttempt: number;
  hasBeenVisible: boolean;
  pendingAttempt: number | null;
  pendingRect: OsRect | null;
  resizeMax: { width: number; height: number } | null;
  rollbackAttempt: number | null;
}

type OsWindowInteractionEvents = {
  authoritativeRectApplied: { rect: OsRect };
  disposed: Record<never, never>;
  gestureAttemptStarted: Record<never, never>;
  gestureCompletionSettled: { attempt: number };
  rectHeld: { attempt: number; rect: OsRect };
  rndRegistered: { instance: Rnd | null };
  resizeMaximumChanged: { maximum: { width: number; height: number } | null };
  rollbackApplied: { attempt: number };
  rollbackRequested: { attempt: number };
  visibilityObserved: { visible: boolean };
};

const rndByTrigger = new WeakMap<object, Rnd>();

function applyAuthoritativeRect(trigger: object, rect: OsRect): void {
  const rnd = rndByTrigger.get(trigger);
  if (!rnd) return;
  try {
    rnd.updatePosition({ x: rect.x, y: rect.y });
    rnd.updateSize({ width: rect.w, height: rect.h });
  } catch (error) {
    console.error("Failed to restore an authoritative window rectangle", error);
  }
}

export const osWindowInteractionLogic = createStoreLogic<
  OsWindowInteractionState,
  OsWindowInteractionEvents,
  never,
  { visible: boolean }
>({
  context: input => ({
    gestureAttempt: 0,
    hasBeenVisible: input.visible,
    pendingAttempt: null,
    pendingRect: null,
    resizeMax: null,
    rollbackAttempt: null,
  }),
  on: {
    rndRegistered: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        if (event.instance === null) rndByTrigger.delete(trigger);
        else rndByTrigger.set(trigger, event.instance);
      });
      return context;
    },
    authoritativeRectApplied: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => applyAuthoritativeRect(trigger, event.rect));
      return context;
    },
    disposed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => rndByTrigger.delete(trigger));
      return context;
    },
    visibilityObserved: (context, event) =>
      !event.visible || context.hasBeenVisible ? undefined : { ...context, hasBeenVisible: true },
    gestureAttemptStarted: context => ({
      ...context,
      gestureAttempt: context.gestureAttempt + 1,
      rollbackAttempt: null,
    }),
    rectHeld: (context, event) =>
      event.attempt !== context.gestureAttempt
        ? undefined
        : { ...context, pendingAttempt: event.attempt, pendingRect: event.rect },
    gestureCompletionSettled: (context, event) =>
      context.pendingAttempt !== event.attempt
        ? undefined
        : { ...context, pendingAttempt: null, pendingRect: null },
    rollbackRequested: (context, event) =>
      event.attempt !== context.gestureAttempt
        ? undefined
        : {
            ...context,
            pendingAttempt: null,
            pendingRect: null,
            rollbackAttempt: event.attempt,
          },
    rollbackApplied: (context, event) =>
      context.rollbackAttempt !== event.attempt ? undefined : { ...context, rollbackAttempt: null },
    resizeMaximumChanged: (context, event) => ({ ...context, resizeMax: event.maximum }),
  },
});
