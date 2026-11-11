import { useWorkerPool } from "@pierre/diffs/react";
import { act, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CodeHighlightPoolProvider } from "../code-highlight-pool";
import {
  CODE_HIGHLIGHT_POOL_IDLE_MS,
  inspectCodeHighlightPool,
  resolveCodeHighlightPoolSize,
} from "../code-highlight-pool-store";

// The worker is the I/O boundary: a stand-in that accepts messages and never answers.
function fakeWorker(terminate: () => void): Worker {
  return {
    postMessage() {},
    addEventListener() {},
    removeEventListener() {},
    dispatchEvent: () => false,
    terminate,
    onmessage: null,
    onmessageerror: null,
    onerror: null,
  } as unknown as Worker;
}

afterEach(() => {
  vi.useRealTimers();
});

describe("CodeHighlightPoolProvider", () => {
  it("Should share one pool across providers and terminate it after the idle window", () => {
    vi.useFakeTimers();
    const terminate = vi.fn();
    const workerFactory = vi.fn(() => fakeWorker(terminate));

    const first = render(<CodeHighlightPoolProvider workerFactory={workerFactory} />);
    const second = render(<CodeHighlightPoolProvider workerFactory={workerFactory} />);
    expect(inspectCodeHighlightPool()).toEqual({ consumers: 2, active: true });

    first.unmount();
    second.unmount();
    expect(inspectCodeHighlightPool()).toEqual({ consumers: 0, active: true });

    act(() => vi.advanceTimersByTime(CODE_HIGHLIGHT_POOL_IDLE_MS - 1));
    expect(inspectCodeHighlightPool().active).toBe(true);
    act(() => vi.advanceTimersByTime(1));
    expect(inspectCodeHighlightPool().active).toBe(false);
  });

  it("Should hand the pool to viewers on their first render", () => {
    vi.useFakeTimers();
    const firstRenderPools: unknown[] = [];
    function Viewer() {
      const pool = useWorkerPool();
      if (firstRenderPools.length === 0) firstRenderPools.push(pool);
      return null;
    }
    const view = render(
      <CodeHighlightPoolProvider workerFactory={() => fakeWorker(() => {})}>
        <Viewer />
      </CodeHighlightPoolProvider>
    );
    expect(firstRenderPools[0]).toBeDefined();
    view.unmount();
    act(() => vi.advanceTimersByTime(CODE_HIGHLIGHT_POOL_IDLE_MS));
    expect(inspectCodeHighlightPool().active).toBe(false);
  });

  it("Should keep highlighting on the main thread without a worker factory", () => {
    render(<CodeHighlightPoolProvider />);
    expect(inspectCodeHighlightPool().active).toBe(false);
  });

  it("Should size the pool between two and four workers", () => {
    expect(resolveCodeHighlightPoolSize(1)).toBe(2);
    expect(resolveCodeHighlightPoolSize(6)).toBe(3);
    expect(resolveCodeHighlightPoolSize(32)).toBe(4);
    expect(resolveCodeHighlightPoolSize(undefined)).toBe(2);
  });
});
