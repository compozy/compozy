// Suite: workspace-switch hydration barrier
// Invariant: selecting a different workspace enters hydration synchronously before route effects run.
// Owning layer: active-workspace store ↔ routing coordinator boundary.
import { afterEach, describe, expect, it, vi } from "vitest";

import { useActiveWorkspaceStore } from "@/systems/workspace/hooks/use-active-workspace-store";

import { subscribeWorkspaceSwitchBarrier } from "../workspace-switch-barrier";

afterEach(() => {
  useActiveWorkspaceStore.setState({ selectedWorkspaceId: null });
});

describe("subscribeWorkspaceSwitchBarrier", () => {
  it("Should enter the barrier inside the workspace selection update", () => {
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "workspace:one" });
    const order: string[] = [];
    const beginWorkspaceSwitch = vi.fn(() => order.push("barrier"));
    const unsubscribe = subscribeWorkspaceSwitchBarrier({ beginWorkspaceSwitch });

    useActiveWorkspaceStore.getState().setSelectedWorkspaceId("workspace:two");
    order.push("after-selection");

    expect(order).toEqual(["barrier", "after-selection"]);
    expect(beginWorkspaceSwitch).toHaveBeenCalledOnce();

    useActiveWorkspaceStore.getState().setSelectedWorkspaceId("workspace:two");
    expect(beginWorkspaceSwitch).toHaveBeenCalledOnce();
    unsubscribe();
  });
});
