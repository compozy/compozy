import { createStore } from "@xstate/store";
import { describe, expect, it, vi } from "vitest";

import { runtimeFavoritesConfig } from "../runtime-favorites-store";
import { runtimeSelectorPopupLogic } from "../runtime-selector-store";

// Suite: runtime selector XState transitions
// Invariant: closed popups expose no list interaction state, and persisted
// preferences are shared across selector catalogs without accepting foreign
// mutations from any individual catalog.
// Boundary IN: Popup and local-preference events.
// Boundary OUT: Derived list/ARIA projection and browser persistence adapter.
describe("runtime selector stores", () => {
  it("Should reset popup-only filter, query, cursor, and announcement when closed", () => {
    const store = runtimeSelectorPopupLogic.createStore();
    const [opened] = store.transition(store.getInitialSnapshot(), { type: "popupOpened" });
    const [filtered] = store.transition(opened, { type: "railChanged", railFilter: "codex" });
    const [searched] = store.transition(filtered, { type: "queryChanged", query: "gpt" });
    const [highlighted] = store.transition(searched, {
      type: "activeRowChanged",
      row: { cursor: "group:codex:gpt", key: "codex:gpt" },
    });
    const [announced] = store.transition(highlighted, {
      type: "favoriteAnnounced",
      message: "Favorited GPT from Codex",
    });
    const [closed] = store.transition(announced, { type: "popupClosed" });

    expect(announced.context).toEqual({
      phase: "open",
      railFilter: "codex",
      query: "gpt",
      activeRow: { cursor: "group:codex:gpt", key: "codex:gpt" },
      favoriteAnnouncement: "Favorited GPT from Codex",
    });
    expect(closed.context).toEqual({ phase: "closed" });
  });

  it("Should preserve cross-catalog preferences and reject catalog-foreign mutations", () => {
    const store = createStore(runtimeFavoritesConfig);
    store.trigger.storageHydrated({
      favorites: ["codex:gpt", "codex:gpt", "foreign"],
      recents: ["foreign", "codex:gpt", "codex:gpt"],
    });
    const notified = vi.fn();
    const subscription = store.subscribe(notified);
    notified.mockClear();
    store.trigger.favoriteToggled({
      key: "foreign",
      validKeys: ["codex:gpt"],
    });

    expect(notified).not.toHaveBeenCalled();
    expect(store.getSnapshot().context).toEqual({
      favorites: ["codex:gpt", "foreign"],
      recents: ["foreign", "codex:gpt"],
    });

    store.trigger.recentPushed({ key: "codex:gpt", validKeys: ["codex:gpt"] });
    expect(notified).toHaveBeenCalledOnce();
    expect(store.getSnapshot().context).toEqual({
      favorites: ["codex:gpt", "foreign"],
      recents: ["codex:gpt", "foreign"],
    });
    subscription.unsubscribe();
  });
});
