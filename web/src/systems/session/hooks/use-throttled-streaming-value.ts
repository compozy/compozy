import { useEffect } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { HIGHLIGHT_THROTTLE_MS } from "../lib/session-smooth-reveal";
import { throttledStreamingValueLogic } from "./throttled-streaming-value-store";

/**
 * A streamed string an expensive consumer follows on its own cadence (ADR-008:
 * the highlighter re-tokenizes a growing code block on a slower throttle than
 * the prose reveal). Inactive, the value passes through unchanged.
 */
export function useThrottledStreamingValue(
  value: string,
  active: boolean,
  intervalMs: number = HIGHLIGHT_THROTTLE_MS
): string {
  const store = useStore(throttledStreamingValueLogic, { value });
  const committed = useSelector(store, snapshot => snapshot.context.committed);

  useEffect(() => {
    store.trigger.valueObserved({ active, at: performance.now(), intervalMs, value });
  }, [active, intervalMs, store, value]);

  useEffect(() => () => store.trigger.disposed(), [store]);

  return active ? committed : value;
}
