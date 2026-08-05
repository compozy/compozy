import { useEffect } from "react";
import { createStoreLogic } from "@xstate/store";
import { useStore } from "@xstate/store-react";

interface AnimationFrameLatest<T> {
  cancel: () => void;
  schedule: (event: { value: T }) => void;
}

interface AnimationFrameLatestContext {
  onFrame: (value: unknown) => void;
  scheduled: boolean;
}

interface AnimationFrameLatestInput {
  onFrame: (value: unknown) => void;
}

type AnimationFrameLatestEventPayloadMap = {
  callbackChanged: { onFrame: (value: unknown) => void };
  canceled: {};
  frameFlushed: {};
  scheduled: { value: unknown };
};

interface AnimationFrameRuntime {
  frameID: number | null;
  pending: { value: unknown } | null;
}

const animationFrameRuntimes = new WeakMap<object, AnimationFrameRuntime>();

const animationFrameLatestLogic = createStoreLogic<
  AnimationFrameLatestContext,
  AnimationFrameLatestEventPayloadMap,
  {},
  AnimationFrameLatestInput
>({
  context: input => ({ onFrame: input.onFrame, scheduled: false }),
  on: {
    callbackChanged: (context, event) => ({ ...context, onFrame: event.onFrame }),
    canceled: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        const runtime = animationFrameRuntimes.get(trigger);
        if (runtime?.frameID !== null && runtime?.frameID !== undefined) {
          cancelAnimationFrame(runtime.frameID);
        }
        animationFrameRuntimes.delete(trigger);
      });
      return { ...context, scheduled: false };
    },
    frameFlushed: context => ({ ...context, scheduled: false }),
    scheduled: (context, event, enqueue) => {
      enqueue.effect(({ getSnapshot, trigger }) => {
        let runtime = animationFrameRuntimes.get(trigger);
        if (runtime === undefined) {
          runtime = { frameID: null, pending: null };
          animationFrameRuntimes.set(trigger, runtime);
        }
        runtime.pending = { value: event.value };
        if (runtime.frameID !== null) return;
        runtime.frameID = requestAnimationFrame(() => {
          runtime.frameID = null;
          const pending = runtime.pending;
          runtime.pending = null;
          trigger.frameFlushed();
          if (pending !== null) getSnapshot().context.onFrame(pending.value);
        });
      });
      return { ...context, scheduled: true };
    },
  },
});

/** Runs at most once per animation frame with the latest scheduled value. */
export function useAnimationFrameLatest<T>(onFrame: (value: T) => void): AnimationFrameLatest<T> {
  const untypedOnFrame = onFrame as (value: unknown) => void;
  const store = useStore(animationFrameLatestLogic, { onFrame: untypedOnFrame });

  useEffect(() => {
    store.trigger.callbackChanged({ onFrame: untypedOnFrame });
  }, [store, untypedOnFrame]);
  useEffect(() => () => store.trigger.canceled(), [store]);

  return {
    cancel: store.trigger.canceled,
    schedule: store.trigger.scheduled,
  };
}
