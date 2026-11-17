/** Records callback events and waits for a particular event without polling. */
export function createEventRecorder<T>() {
  const events: T[] = [];
  const pending = new Map<number, PromiseWithResolvers<void>>();
  return {
    events,
    record(event: T) {
      const count = events.push(event);
      pending.get(count)?.resolve();
      pending.delete(count);
    },
    waitForCount(count: number): Promise<void> {
      if (events.length >= count) return Promise.resolve();
      let signal = pending.get(count);
      if (!signal) {
        signal = Promise.withResolvers<void>();
        pending.set(count, signal);
      }
      return signal.promise;
    },
  };
}
