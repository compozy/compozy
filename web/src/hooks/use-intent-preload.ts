import { useEffect, useRef } from "react";

const INTENT_PRELOAD_DELAY_MS = 100;

/** Speculative reads wait for a settled hover/focus; touch starts before the click. */
export function useIntentPreload(preload: () => void | Promise<unknown>, enabled = true) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current !== null) clearTimeout(timer.current);
      timer.current = null;
    },
    [enabled, preload]
  );

  const cancel = () => {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
  };
  const run = () => {
    cancel();
    if (enabled)
      void Promise.resolve()
        .then(preload)
        .catch(() => undefined);
  };
  const schedule = () => {
    if (!enabled || timer.current !== null) return;
    timer.current = setTimeout(run, INTENT_PRELOAD_DELAY_MS);
  };

  return {
    onPointerEnter: schedule,
    onPointerLeave: cancel,
    onFocus: schedule,
    onBlur: cancel,
    onTouchStart: run,
  };
}
