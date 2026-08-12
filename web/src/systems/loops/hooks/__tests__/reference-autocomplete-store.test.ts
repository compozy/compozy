// Suite: reference autocomplete lifecycle
// Invariant: delayed blur dismissal runs once only while still owned; refresh, selection, and
// disposal cancel it before it can publish another transition.
// Owning layer: reference autocomplete interaction store. Boundary OUT: DOM focus/caret wiring.
import { afterEach, describe, expect, it, vi } from "vitest";

import { referenceAutocompleteLogic } from "../reference-autocomplete-store";

afterEach(() => {
  vi.useRealTimers();
});

describe("referenceAutocompleteLogic", () => {
  it("Should dismiss an active query after the blur delay", () => {
    vi.useFakeTimers();
    const store = referenceAutocompleteLogic.createStore();
    store.trigger.refreshed({ query: "inputs" });
    store.trigger.blurred();

    vi.advanceTimersByTime(119);
    expect(store.getSnapshot().context.query).toBe("inputs");
    vi.advanceTimersByTime(1);
    expect(store.getSnapshot().context.query).toBeNull();
  });

  it.each([
    [
      "refresh",
      (store: ReturnType<typeof referenceAutocompleteLogic.createStore>) =>
        store.trigger.refreshed({ query: "nodes" }),
    ],
    [
      "selection",
      (store: ReturnType<typeof referenceAutocompleteLogic.createStore>) =>
        store.trigger.selected({ caret: 12 }),
    ],
    [
      "disposal",
      (store: ReturnType<typeof referenceAutocompleteLogic.createStore>) =>
        store.trigger.disposed(),
    ],
  ])("Should cancel delayed dismissal after %s", (_case, replaceOwnerState) => {
    vi.useFakeTimers();
    const store = referenceAutocompleteLogic.createStore();
    store.trigger.refreshed({ query: "inputs" });
    store.trigger.blurred();
    replaceOwnerState(store);
    const settledSnapshot = store.getSnapshot();

    vi.advanceTimersByTime(120);

    expect(store.getSnapshot()).toBe(settledSnapshot);
  });
});
