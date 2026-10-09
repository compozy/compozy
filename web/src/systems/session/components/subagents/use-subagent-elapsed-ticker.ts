import { formatSubagentElapsed } from "./subagent-format";

// One shared 1 s ticker for every live elapsed label on the page. It writes
// `textContent` directly so a tick never re-renders React (t3 `AgentElapsed`).
// The interval runs only while at least one label is attached.
const TICK_MS = 1_000;
const labels = new Map<HTMLElement, number>();
let timer: ReturnType<typeof setInterval> | null = null;

function write(element: HTMLElement, startMs: number, nowMs: number): void {
  const text = formatSubagentElapsed(nowMs - startMs);
  if (element.textContent !== text) element.textContent = text;
}

function tick(): void {
  const nowMs = Date.now();
  for (const [element, startMs] of labels) write(element, startMs, nowMs);
}

function attach(element: HTMLElement, startMs: number): () => void {
  labels.set(element, startMs);
  write(element, startMs, Date.now());
  timer ??= setInterval(tick, TICK_MS);
  return () => {
    labels.delete(element);
    if (labels.size === 0 && timer !== null) {
      clearInterval(timer);
      timer = null;
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
