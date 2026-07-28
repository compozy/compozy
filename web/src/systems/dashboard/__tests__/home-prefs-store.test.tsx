import { rehydrateStore } from "@xstate/store/persist";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { homePrefsStore } from "../hooks/use-home-prefs-store";

// Suite: Home browser preference ownership.
// Invariant: persisted preferences normalize on hydration and unrelated
// preference updates do not invalidate a selected projection.
// Boundary IN: local preference events and persisted browser state.
// Boundary OUT: the shared preference store and its React selectors.
describe("home preferences store", () => {
  beforeEach(() => {
    window.localStorage.removeItem("compozy:home-prefs:v2");
    homePrefsStore.trigger.usageWindowSelected({ usageWindow: 30 });
    homePrefsStore.trigger.systemPanelClosed();
  });

  it("Should round-trip normalized persisted preferences", async () => {
    homePrefsStore.trigger.usageWindowSelected({ usageWindow: 7 });
    homePrefsStore.trigger.systemPanelOpened();

    expect(JSON.parse(window.localStorage.getItem("compozy:home-prefs:v2") ?? "")).toEqual({
      context: { usageWindow: 7, systemOpen: true },
      version: 0,
    });

    const persisted = window.localStorage.getItem("compozy:home-prefs:v2");
    expect(persisted).not.toBeNull();
    homePrefsStore.trigger.usageWindowSelected({ usageWindow: 90 });
    window.localStorage.setItem("compozy:home-prefs:v2", persisted ?? "");
    await rehydrateStore(homePrefsStore);
    expect(homePrefsStore.getSnapshot().context).toEqual({ usageWindow: 7, systemOpen: true });

    window.localStorage.setItem(
      "compozy:home-prefs:v2",
      JSON.stringify({ context: { usageWindow: 13, systemOpen: false }, version: 0 })
    );
    await rehydrateStore(homePrefsStore);
    expect(homePrefsStore.getSnapshot().context).toEqual({ usageWindow: 30, systemOpen: false });
  });

  it("Should keep the usage selector stable when the system panel changes", () => {
    homePrefsStore.trigger.usageWindowSelected({ usageWindow: 7 });
    const usageChanged = vi.fn();
    const usageWindow = homePrefsStore.select(context => context.usageWindow);
    const subscription = usageWindow.subscribe(usageChanged);
    usageChanged.mockClear();

    homePrefsStore.trigger.systemPanelOpened();

    expect(usageChanged).not.toHaveBeenCalled();
    expect(usageWindow.get()).toBe(7);

    homePrefsStore.trigger.usageWindowSelected({ usageWindow: 90 });
    expect(usageChanged).toHaveBeenCalledOnce();
    expect(usageWindow.get()).toBe(90);
    subscription.unsubscribe();
  });
});
