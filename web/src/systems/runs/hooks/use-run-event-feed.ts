import { useMemo, useSyncExternalStore } from "react";

import { createRunEventStore, type RunEventStore, type RunFeedEvent } from "../lib/event-store";

export interface UseRunEventFeedResult {
  events: readonly RunFeedEvent[];
  append: (eventId: string | null, raw: unknown) => RunFeedEvent | null;
  reset: () => void;
}

export function useRunEventFeed(runId: string | null): UseRunEventFeedResult {
  const store: RunEventStore = useMemo(() => createRunEventStore(), [runId]);
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
