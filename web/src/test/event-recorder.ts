export function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((settle, fail) => {
    resolve = settle;
    reject = fail;
  });
  return { promise, resolve, reject };
}

/** Records callback events and waits for a particular event without polling. */
export function createEventRecorder<T>() {
  const events: T[] = [];
  const pending = new Map<number, ReturnType<typeof createDeferred<void>>>();
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
        signal = createDeferred<void>();
        pending.set(count, signal);
      }
      return signal.promise;
    },
  };
}
