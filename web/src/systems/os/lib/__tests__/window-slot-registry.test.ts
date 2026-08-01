import { describe, expect, it } from "vitest";

import {
  ensureWindowSlotStore,
  pruneWindowSlotStores,
  subscribeWindowSlotRegistry,
  windowSlotRegistryVersion,
} from "../window-slot-registry";

describe("window slot registry", () => {
  it("Should notify existing subscribers only when the observable slot changes", () => {
    const windowId = "registry-created-window";
    pruneWindowSlotStores(new Set());
    const initialVersion = windowSlotRegistryVersion();
    let notifications = 0;
    const unsubscribe = subscribeWindowSlotRegistry(() => {
      notifications += 1;
    });

    try {
      const store = ensureWindowSlotStore(windowId);

      expect(notifications).toBe(0);
      expect(windowSlotRegistryVersion()).toBe(initialVersion);

      store.publishSlot({}, { crumb: "Tasks" });

      expect(notifications).toBe(1);
      expect(windowSlotRegistryVersion()).toBe(initialVersion + 1);
    } finally {
      unsubscribe();
      pruneWindowSlotStores(new Set());
    }
  });
});
