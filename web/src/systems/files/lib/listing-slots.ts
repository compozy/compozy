/** Concurrent folder listings per page; the rest wait their turn (perf audit P-5). */
export const LISTING_CONCURRENCY = 6;

let active = 0;
const waiting: (() => void)[] = [];

function release(): void {
  active -= 1;
  waiting.shift()?.();
}

/**
 * Runs `task` once one of `LISTING_CONCURRENCY` slots is free. An aborted wait leaves the queue
 * without taking a slot, so a cancelled listing never blocks the next one.
 */
export async function withListingSlot<T>(task: () => Promise<T>, signal?: AbortSignal): Promise<T> {
  if (active >= LISTING_CONCURRENCY) {
    await new Promise<void>((resolve, reject) => {
      const start = () => {
        signal?.removeEventListener("abort", abort);
        resolve();
      };
      const abort = () => {
        const index = waiting.indexOf(start);
        if (index >= 0) waiting.splice(index, 1);
        reject(signal?.reason);
      };
      if (signal?.aborted) {
        abort();
        return;
      }
      waiting.push(start);
      signal?.addEventListener("abort", abort, { once: true });
    });
  }
  active += 1;
  try {
    return await task();
  } finally {
    release();
  }
}
