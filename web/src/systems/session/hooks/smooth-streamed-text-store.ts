import { createStoreLogic } from "@xstate/store";

import {
  createSmoothRevealState,
  isAppendOnlyUpdate,
  type SmoothRevealState,
  stepSmoothReveal,
} from "../lib/session-smooth-reveal";

interface SmoothTextState {
  target: string;
  emittedCount: number;
  reveal: SmoothRevealState;
  active: boolean;
  generation: number;
}

type SmoothTextEvents = {
  textObserved: { text: string; active: boolean };
  frameElapsed: { generation: number; at: number };
  disposed: Record<never, never>;
};

interface FrameTrigger {
  frameElapsed: (event: SmoothTextEvents["frameElapsed"]) => void;
}

const framesByTrigger = new WeakMap<object, number>();

function clearFrame(trigger: object): void {
  const frame = framesByTrigger.get(trigger);
  if (frame !== undefined) cancelAnimationFrame(frame);
  framesByTrigger.delete(trigger);
}

function scheduleFrame(trigger: FrameTrigger, generation: number): void {
  if (framesByTrigger.has(trigger)) return;
  const frame = requestAnimationFrame(at => {
    // Cancellation may race a callback already queued by the browser. Only the
    // currently owned frame may release the handle or advance this clock.
    if (framesByTrigger.get(trigger) !== frame) return;
    framesByTrigger.delete(trigger);
    trigger.frameElapsed({ generation, at });
  });
  framesByTrigger.set(trigger, frame);
}

/** The animation clock owns progress; rendered text is derived from the current target and count. */
export const smoothStreamedTextLogic = createStoreLogic<
  SmoothTextState,
  SmoothTextEvents,
  never,
  { text: string }
>({
  context: ({ text }) => ({
    target: text,
    emittedCount: text.length,
    reveal: createSmoothRevealState(text.length),
    active: false,
    generation: 0,
  }),
  on: {
    textObserved: (context, event, enqueue) => {
      const reset = !event.active || !isAppendOnlyUpdate(context.target, event.text);
      if (reset) {
        enqueue.effect(({ trigger }) => clearFrame(trigger));
        return {
          target: event.text,
          emittedCount: event.text.length,
          reveal: createSmoothRevealState(event.text.length),
          active: event.active,
          generation: context.generation + 1,
        };
      }
      if (event.text.length > context.reveal.shown) {
        enqueue.effect(({ trigger }) => scheduleFrame(trigger, context.generation));
      }
      return { ...context, target: event.text, active: event.active };
    },
    frameElapsed: (context, event, enqueue) => {
      if (!context.active || event.generation !== context.generation) return;
      const reveal = { ...context.reveal };
      const step = stepSmoothReveal(reveal, event.at, context.target.length, context.emittedCount);
      if (!step.done) {
        enqueue.effect(({ trigger }) => scheduleFrame(trigger, context.generation));
      }
      return { ...context, reveal, emittedCount: step.emitCount ?? context.emittedCount };
    },
    disposed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => clearFrame(trigger));
      return { ...context, active: false, generation: context.generation + 1 };
    },
  },
});
