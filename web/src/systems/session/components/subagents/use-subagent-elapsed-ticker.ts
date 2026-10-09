import { formatDuration } from "@compozy/ui";

import { subscribeSecondClock } from "@/hooks/use-second-clock";

// Every live elapsed label rides the page's one 1 Hz clock and writes its own
// `textContent`, so a tick never re-renders React (t3 `AgentElapsed`). The
// clock subscription lives only while at least one label is attached.
const labels = new Map<HTMLElement, number>();
let unsubscribe: (() => void) | null = null;

function write(element: HTMLElement, startMs: number, nowMs: number): void {
  const text = formatDuration(nowMs - startMs, { padded: true });
  if (element.textContent !== text) element.textContent = text;
}

function tick(nowMs: number): void {
  for (const [element, startMs] of labels) write(element, startMs, nowMs);
}

function attach(element: HTMLElement, startMs: number): () => void {
  labels.set(element, startMs);
  write(element, startMs, Date.now());
  unsubscribe ??= subscribeSecondClock(tick);
  return () => {
    labels.delete(element);
    if (labels.size === 0 && unsubscribe !== null) {
      unsubscribe();
      unsubscribe = null;
    }
  };
}

/**
 * Ref callback that keeps an element's text at the elapsed time since the
 * daemon's `startMs`. The element must render no React children of its own.
 */
export function useSubagentElapsedTicker(
  startMs: number
): (element: HTMLElement | null) => (() => void) | undefined {
  return element => (element ? attach(element, startMs) : undefined);
}
