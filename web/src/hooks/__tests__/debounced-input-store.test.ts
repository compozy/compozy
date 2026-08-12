// Suite: debounced input interaction store
// Invariant: only the latest draft commits, and reset/disposal cancel pending work.
// Owning layer: shared debounced-input store. Boundary OUT: route/query consumers.
import { afterEach, describe, expect, it, vi } from "vitest";

import { debouncedInputLogic } from "../debounced-input-store";

afterEach(() => {
  vi.useRealTimers();
});

describe("debouncedInputLogic", () => {
  it("Should commit only the latest value in an input burst", () => {
    vi.useFakeTimers();
    const store = debouncedInputLogic.createStore({ value: "" });
    const commit = vi.fn();

    store.trigger.valueChanged({ commit, delayMs: 180, value: "a" });
    store.trigger.valueChanged({ commit, delayMs: 180, value: "ab" });
    store.trigger.valueChanged({ commit, delayMs: 180, value: "abc" });
    vi.advanceTimersByTime(179);
    expect(commit).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);

    expect(commit).toHaveBeenCalledOnce();
    expect(commit).toHaveBeenCalledWith("abc");
    expect(store.getSnapshot().context.committedValue).toBe("abc");
  });

  it("Should cancel pending work when an external value replaces the draft", () => {
    vi.useFakeTimers();
    const store = debouncedInputLogic.createStore({ value: "before" });
    const commit = vi.fn();
    store.trigger.valueChanged({ commit, delayMs: 180, value: "pending" });

    store.trigger.externalValueObserved({ value: "from-history" });
    vi.runAllTimers();

    expect(commit).not.toHaveBeenCalled();
    expect(store.getSnapshot().context).toMatchObject({
      committedValue: "from-history",
      draftValue: "from-history",
      phase: "idle",
    });
  });

  it("Should cancel pending work when the owner is disposed", () => {
    vi.useFakeTimers();
    const store = debouncedInputLogic.createStore({ value: "" });
    const commit = vi.fn();
    store.trigger.valueChanged({ commit, delayMs: 180, value: "pending" });

    store.trigger.disposed();
    vi.runAllTimers();

    expect(commit).not.toHaveBeenCalled();
  });
});
