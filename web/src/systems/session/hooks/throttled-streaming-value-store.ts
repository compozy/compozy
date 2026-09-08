import { createStoreLogic } from "@xstate/store";

import { planThrottledCommit } from "../lib/session-smooth-reveal";

interface ThrottledStreamingValueState {
  committed: string;
  latest: string;
  generation: number;
  /** 0 = fresh burst: the next change passes at once. */
  lastCommitAt: number;
  pending: boolean;
}

type ThrottledStreamingValueEvents = {
  valueObserved: { value: string; active: boolean; intervalMs: number; at: number };
  commitElapsed: { generation: number; at: number };
  disposed: Record<never, never>;
};

const timersByTrigger = new WeakMap<object, ReturnType<typeof setTimeout>>();

function clearTimer(trigger: object): void {
  const timer = timersByTrigger.get(trigger);
  if (timer !== undefined) clearTimeout(timer);
  timersByTrigger.delete(trigger);
}

/**
 * The throttled value an expensive consumer follows (ADR-008 highlighter
 * cadence). The first change of a burst commits at once, later changes wait
 * for the interval and the trailing edge always lands; deactivation commits the
 * latest value immediately so settled content never lags. Timers live in store
 * effects, never in render or component effects.
 */
export const throttledStreamingValueLogic = createStoreLogic<
  ThrottledStreamingValueState,
  ThrottledStreamingValueEvents,
  never,
  { value: string }
>({
  context: input => ({
    committed: input.value,
    latest: input.value,
    generation: 0,
    lastCommitAt: 0,
    pending: false,
  }),
  on: {
    valueObserved: (context, event, enqueue) => {
      if (!event.active) {
        enqueue.effect(({ trigger }) => clearTimer(trigger));
        return {
          ...context,
          committed: event.value,
          latest: event.value,
          generation: context.generation + 1,
          lastCommitAt: 0,
          pending: false,
        };
      }
      if (event.value === context.latest) return;
      const plan = planThrottledCommit(context.lastCommitAt, event.at, event.intervalMs);
      if (plan.immediate) {
        enqueue.effect(({ trigger }) => clearTimer(trigger));
        return {
          ...context,
          committed: event.value,
          latest: event.value,
          generation: context.generation + 1,
          lastCommitAt: event.at,
          pending: false,
        };
      }
      // A pending trailing commit reads the latest value when it fires.
      if (context.pending) return { ...context, latest: event.value };
      const generation = context.generation + 1;
      enqueue.effect(({ trigger }) => {
        clearTimer(trigger);
        timersByTrigger.set(
          trigger,
          setTimeout(
            () => trigger.commitElapsed({ generation, at: performance.now() }),
            plan.delayMs
          )
        );
      });
      return { ...context, generation, latest: event.value, pending: true };
    },
    commitElapsed: (context, event) => {
      if (event.generation !== context.generation || !context.pending) return;
      return {
        ...context,
        committed: context.latest,
        lastCommitAt: event.at,
        pending: false,
      };
    },
    disposed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => clearTimer(trigger));
      return { ...context, pending: false };
    },
  },
});
