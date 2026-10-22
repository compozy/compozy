// Suite: workspace-switch hydration barrier
// Invariant: changing the destination's desktop partition — another project, or project ↔ User —
// enters hydration synchronously before route effects run; a change that keeps the partition does not.
// Owning layer: active-workspace store ↔ routing coordinator boundary.
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  enableAllProjects,
  focusUser,
  setActiveWorkspaceId,
} from "@/systems/workspace/stores/active-workspace-store";

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

  it("Should keep routing live when selection resolves to the already-bound workspace", () => {
    const beginWorkspaceSwitch = vi.fn();
    const unsubscribe = subscribeWorkspaceSwitchBarrier(
      { beginWorkspaceSwitch },
      () => "workspace:one"
    );

    setActiveWorkspaceId("workspace:one");

    expect(beginWorkspaceSwitch).not.toHaveBeenCalled();
    unsubscribe();
  });

  it("Should enter the barrier when focus moves between a project and User", () => {
    setActiveWorkspaceId("workspace:one");
    const beginWorkspaceSwitch = vi.fn();
    const unsubscribe = subscribeWorkspaceSwitchBarrier({ beginWorkspaceSwitch });

    focusUser();
    expect(beginWorkspaceSwitch).toHaveBeenCalledOnce();

    setActiveWorkspaceId("workspace:one");
    expect(beginWorkspaceSwitch).toHaveBeenCalledTimes(2);
    unsubscribe();
  });

  it("Should enter the barrier when All projects moves to the global desktop", () => {
    setActiveWorkspaceId("workspace:one");
    const beginWorkspaceSwitch = vi.fn();
    const unsubscribe = subscribeWorkspaceSwitchBarrier(
      { beginWorkspaceSwitch },
      () => "workspace:one"
    );

    enableAllProjects();

    expect(beginWorkspaceSwitch).toHaveBeenCalledOnce();
    unsubscribe();
  });

  it("Should keep routing live when the global desktop is already bound", () => {
    setActiveWorkspaceId("workspace:one");
    const beginWorkspaceSwitch = vi.fn();
    const unsubscribe = subscribeWorkspaceSwitchBarrier({ beginWorkspaceSwitch }, () => "global");

    enableAllProjects();

    expect(beginWorkspaceSwitch).not.toHaveBeenCalled();
    unsubscribe();
  });
});
