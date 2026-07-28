// Suite: workspace-switch hydration barrier
// Invariant: selecting a different workspace enters hydration synchronously before route effects run.
// Owning layer: active-workspace store ↔ routing coordinator boundary.
import { afterEach, describe, expect, it, vi } from "vitest";

import { setActiveWorkspaceId } from "@/systems/workspace/stores/active-workspace-store";

import { subscribeWorkspaceSwitchBarrier } from "../workspace-switch-barrier";

afterEach(() => {
  setActiveWorkspaceId(null);
});

describe("subscribeWorkspaceSwitchBarrier", () => {
  it("Should enter the barrier inside the workspace selection update", () => {
    setActiveWorkspaceId("workspace:one");
    const order: string[] = [];
    const beginWorkspaceSwitch = vi.fn(() => order.push("barrier"));
    const unsubscribe = subscribeWorkspaceSwitchBarrier({ beginWorkspaceSwitch });

    setActiveWorkspaceId("workspace:two");
    order.push("after-selection");

    expect(order).toEqual(["barrier", "after-selection"]);
    expect(beginWorkspaceSwitch).toHaveBeenCalledOnce();

    setActiveWorkspaceId("workspace:two");
    expect(beginWorkspaceSwitch).toHaveBeenCalledOnce();
    unsubscribe();
  });
});
