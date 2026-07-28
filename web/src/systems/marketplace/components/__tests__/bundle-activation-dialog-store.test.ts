import { describe, expect, it, vi } from "vitest";

import { createBundleActivationDialogLogic } from "../bundle-activation-dialog-store";

describe("bundle activation dialog store", () => {
  const pendingPreview = () => new Promise<void>(() => undefined);
  const pendingActivation = () => new Promise<void>(() => undefined);

  it("Should fence a stale preview result after the profile changes", () => {
    const logic = createBundleActivationDialogLogic();
    const store = logic.createStore({
      profile: "default",
      scope: "workspace",
      confirmNetworkRequirement: false,
    });
    let snapshot = store.getInitialSnapshot();

    [snapshot] = store.transition(snapshot, {
      type: "previewExecutionProvided",
      preview: pendingPreview,
    });
    const firstRequestId = snapshot.context.requestId;
    [snapshot] = store.transition(snapshot, {
      type: "profileSelected",
      preview: pendingPreview,
      profile: "strict",
    });
    [snapshot] = store.transition(snapshot, {
      type: "previewSucceeded",
      requestId: firstRequestId,
    });

    expect(snapshot.context.phase).toBe("previewing");
    expect(snapshot.context.profile).toBe("strict");
  });

  it("Should accept activation only when Query reports the current preview ready", () => {
    const logic = createBundleActivationDialogLogic();
    const store = logic.createStore({
      profile: "default",
      scope: "global",
      confirmNetworkRequirement: false,
    });
    let snapshot = store.getInitialSnapshot();

    [snapshot] = store.transition(snapshot, {
      type: "previewExecutionProvided",
      preview: pendingPreview,
    });
    [snapshot] = store.transition(snapshot, {
      type: "activationSubmitted",
      activate: pendingActivation,
      notifySuccess: vi.fn(),
    });
    expect(snapshot.context.phase).toBe("previewing");

    [snapshot] = store.transition(snapshot, { type: "previewSucceeded", requestId: 1 });
    [snapshot] = store.transition(snapshot, {
      type: "activationSubmitted",
      activate: pendingActivation,
      notifySuccess: vi.fn(),
    });
    expect(snapshot.context.phase).toBe("activating");
  });

  it("Should execute fenced preview and activation requests in the store", async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    const activate = vi.fn().mockResolvedValue(undefined);
    const notifySuccess = vi.fn();
    const logic = createBundleActivationDialogLogic();
    const store = logic.createStore({
      profile: "default",
      scope: "workspace",
      confirmNetworkRequirement: true,
    });
    store.trigger.previewExecutionProvided({ preview });
    expect(preview).toHaveBeenCalledWith({
      profile: "default",
      scope: "workspace",
      confirmNetworkRequirement: true,
    });
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("ready"));

    store.trigger.activationSubmitted({ activate, notifySuccess });
    expect(activate).toHaveBeenCalledWith({
      profile: "default",
      scope: "workspace",
      confirmNetworkRequirement: true,
    });
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("activated"));
    expect(notifySuccess).toHaveBeenCalledOnce();
  });
});
