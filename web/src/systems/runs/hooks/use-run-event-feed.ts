import { useMemo, useRef, useSyncExternalStore } from "react";

import { createRunEventStore, type RunEventStore, type RunFeedEvent } from "../lib/event-store";

export interface UseRunEventFeedResult {
  events: readonly RunFeedEvent[];
  append: (eventId: string | null, raw: unknown) => RunFeedEvent | null;
  reset: () => void;
}

export function useRunEventFeed(runId: string | null): UseRunEventFeedResult {
  const storeRef = useRef<RunEventStore | null>(null);
  const activeRunIdRef = useRef<string | null>(null);
  if (!storeRef.current) {
    storeRef.current = createRunEventStore();
  }
  if (activeRunIdRef.current !== runId) {
    storeRef.current.reset();
    activeRunIdRef.current = runId;
  }
  const store = storeRef.current;
  const events = useSyncExternalStore(store.subscribe, store.getSnapshot, store.getServerSnapshot);
  return useMemo(
    () => ({
      events,
      append: store.append,
      reset: store.reset,
    }),
    [events, store]
  );
}
