// Suite: shared seam drag lifecycle
// Invariant: one captured pointer owns a drag, movement publishes only the latest preview per
// animation frame, and every terminal transition leaves no scheduled preview work.
// Owning layer: shared seam drag interaction store. Boundary OUT: seam geometry and command I/O.
import { afterEach, describe, expect, it, vi } from "vitest";

import { createSeamDragLogic } from "../seam-drag-store";

interface TestSeam {
  id: string;
}

const seam: TestSeam = { id: "root:0" };

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("createSeamDragLogic", () => {
  it("Should ignore a foreign pointer that moves after the captured pointer", () => {
    let frame: FrameRequestCallback | undefined;
    const requestFrame = vi.fn((callback: FrameRequestCallback) => {
      frame = callback;
      return 41;
    });
    vi.stubGlobal("requestAnimationFrame", requestFrame);
    vi.stubGlobal("cancelAnimationFrame", vi.fn());
    const firstPreview = vi.fn();
    const foreignPreview = vi.fn();
    const store = createSeamDragLogic<TestSeam>().createStore();

    store.trigger.dragStarted({ coordinate: 100, pointerId: 7, seam });
    store.trigger.pointerMoved({ coordinate: 115, pointerId: 7, preview: firstPreview });
    store.trigger.pointerMoved({ coordinate: 180, pointerId: 8, preview: foreignPreview });

    expect(requestFrame).toHaveBeenCalledOnce();
    expect(firstPreview).not.toHaveBeenCalled();
    frame?.(16);

    expect(firstPreview).toHaveBeenCalledOnce();
    expect(firstPreview).toHaveBeenCalledWith(seam, 15);
    expect(foreignPreview).not.toHaveBeenCalled();
  });

  it("Should publish the latest delta and schedule another frame after the first completes", () => {
    const frames: FrameRequestCallback[] = [];
    const requestFrame = vi.fn((callback: FrameRequestCallback) => {
      frames.push(callback);
      return frames.length;
    });
    vi.stubGlobal("requestAnimationFrame", requestFrame);
    vi.stubGlobal("cancelAnimationFrame", vi.fn());
    const preview = vi.fn();
    const store = createSeamDragLogic<TestSeam>().createStore();

    store.trigger.dragStarted({ coordinate: 100, pointerId: 7, seam });
    store.trigger.pointerMoved({ coordinate: 115, pointerId: 7, preview });
    store.trigger.pointerMoved({ coordinate: 140, pointerId: 7, preview });

    expect(requestFrame).toHaveBeenCalledOnce();
    frames[0]?.(16);
    expect(preview).toHaveBeenLastCalledWith(seam, 40);

    store.trigger.pointerMoved({ coordinate: 155, pointerId: 7, preview });

    expect(requestFrame).toHaveBeenCalledTimes(2);
    frames[1]?.(32);
    expect(preview).toHaveBeenLastCalledWith(seam, 55);
  });

  it("Should accept only the captured pointer when ending a drag", () => {
    let frame: FrameRequestCallback | undefined;
    const cancelFrame = vi.fn();
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
      frame = callback;
      return 42;
    });
    vi.stubGlobal("cancelAnimationFrame", cancelFrame);
    const preview = vi.fn();
    const store = createSeamDragLogic<TestSeam>().createStore();
    store.trigger.dragStarted({ coordinate: 100, pointerId: 7, seam });
    store.trigger.pointerMoved({ coordinate: 125, pointerId: 7, preview });

    store.trigger.dragEnded({ pointerId: 8 });
    expect(store.getSnapshot().context.phase).toBe("dragging");
    store.trigger.dragEnded({ pointerId: 7 });

    expect(store.getSnapshot().context).toMatchObject({
      pendingDeltaPx: 0,
      phase: "idle",
      pointerId: null,
      seam: null,
    });
    expect(cancelFrame).toHaveBeenCalledWith(42);
    frame?.(16);
    expect(preview).not.toHaveBeenCalled();
  });

  it.each(["cancelled", "disposed"] as const)(
    "Should end the preview and cancel pending work when %s",
    terminalEvent => {
      let frame: FrameRequestCallback | undefined;
      const cancelFrame = vi.fn();
      vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
        frame = callback;
        return 43;
      });
      vi.stubGlobal("cancelAnimationFrame", cancelFrame);
      const preview = vi.fn();
      const endPreview = vi.fn();
      const store = createSeamDragLogic<TestSeam>().createStore();
      store.trigger.dragStarted({ coordinate: 100, pointerId: 7, seam });
      store.trigger.pointerMoved({ coordinate: 125, pointerId: 7, preview });

      if (terminalEvent === "cancelled") store.trigger.dragCancelled({ endPreview });
      else store.trigger.disposed({ endPreview });

      expect(store.getSnapshot().context.phase).toBe("idle");
      expect(cancelFrame).toHaveBeenCalledWith(43);
      expect(endPreview).toHaveBeenCalledOnce();
      frame?.(16);
      expect(preview).not.toHaveBeenCalled();
    }
  );
});
