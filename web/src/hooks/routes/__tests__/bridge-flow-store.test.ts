import { waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn(), warning: vi.fn() },
}));

import { toast } from "sonner";

import {
  bridgeProvidersFixture,
  bridgeVerifyFixture,
  bridgesListFixture,
  createBridgeFixture,
} from "@/systems/bridges/mocks";
import { createBridgeCreateDraft } from "@/systems/bridges";

import { createBridgeCreateFlowLogic } from "../bridge-create-flow-store";
import { createBridgeSetupFlowLogic } from "../bridge-setup-flow-store";

const bridge = bridgesListFixture.bridges[0];
const provider = bridgeProvidersFixture[0];

if (!bridge || !provider) {
  throw new Error("Bridge flow fixtures require one bridge and one provider.");
}

describe("bridge route flow stores", () => {
  it("Should reject a stale create result after the dialog has reopened", () => {
    const store = createBridgeCreateFlowLogic().createStore();
    const create = () => new Promise<typeof createBridgeFixture>(() => undefined);
    const bindSecrets = async () => ({ bound: [], clearedSlotNames: [], failures: {} });
    const navigate = async () => undefined;
    const draft = createBridgeCreateDraft([provider], bridge.workspace_id);
    store.trigger.createOpened({ draft });
    store.trigger.createSubmitted({
      activeWorkspaceId: bridge.workspace_id,
      bindSecrets,
      create,
      navigate,
      provider,
    });
    const staleAttempt = store.getSnapshot().context.attempt;
    store.trigger.createOpened({ draft });
    const notified = vi.fn();
    const subscription = store.subscribe(notified);
    notified.mockClear();

    store.trigger.createCompleted({
      attempt: staleAttempt,
      bindOutcome: { bound: [], clearedSlotNames: [], failures: {} },
      navigate,
      result: createBridgeFixture,
    });

    expect(notified).not.toHaveBeenCalled();
    expect(store.getSnapshot().context.phase).toBe("editing");
    subscription.unsubscribe();
  });

  it("Should retain a committed bridge for secret recovery without retaining plaintext", () => {
    const store = createBridgeCreateFlowLogic().createStore();
    const draft = {
      ...createBridgeCreateDraft([provider], bridge.workspace_id),
      secretSlotValues: { api_key: "plaintext" },
    };
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, { type: "createOpened", draft });
    [snapshot] = store.transition(snapshot, {
      type: "createSubmitted",
      activeWorkspaceId: bridge.workspace_id,
      bindSecrets: async () => ({ bound: [], clearedSlotNames: [], failures: {} }),
      create: async () => createBridgeFixture,
      navigate: async () => undefined,
      provider,
    });
    [snapshot] = store.transition(snapshot, {
      type: "createCompleted",
      attempt: snapshot.context.attempt,
      bindOutcome: {
        bound: [],
        clearedSlotNames: ["api_key"],
        failures: { api_key: "binding failed" },
      },
      navigate: async () => undefined,
      result: createBridgeFixture,
    });

    expect(snapshot.context).toMatchObject({
      phase: "secret-recovery",
      draft: { secretSlotValues: { api_key: "" } },
      recovery: { failures: { api_key: "binding failed" } },
    });
  });

  it("Should not offer create again after a committed bridge fails to open", () => {
    const store = createBridgeCreateFlowLogic().createStore();
    const directProvider = { ...provider, platform: "test" };
    const draft = createBridgeCreateDraft([directProvider], bridge.workspace_id);
    const create = () => new Promise<typeof createBridgeFixture>(() => undefined);
    const bindSecrets = async () => ({ bound: [], clearedSlotNames: [], failures: {} });
    const navigate = () => new Promise<void>(() => undefined);
    store.trigger.createOpened({ draft });
    store.trigger.createSubmitted({
      activeWorkspaceId: bridge.workspace_id,
      bindSecrets,
      create,
      navigate,
      provider: directProvider,
    });
    store.trigger.createCompleted({
      attempt: store.getSnapshot().context.attempt,
      bindOutcome: { bound: [], clearedSlotNames: [], failures: {} },
      navigate,
      result: createBridgeFixture,
    });
    const navigationAttempt = store.getSnapshot().context.attempt;
    store.trigger.navigationFailed({
      attempt: navigationAttempt,
      error: "router unavailable",
    });

    expect(store.getSnapshot().context.phase).toBe("closed");
    const notified = vi.fn();
    const subscription = store.subscribe(notified);
    notified.mockClear();
    store.trigger.createSubmitted({
      activeWorkspaceId: bridge.workspace_id,
      bindSecrets,
      create,
      navigate,
      provider: directProvider,
    });
    expect(notified).not.toHaveBeenCalled();
    expect(store.getSnapshot().context.phase).toBe("closed");
    subscription.unsubscribe();
  });

  it("Should ignore a verification result after setup evidence resets", async () => {
    let resolveVerification!: (result: typeof bridgeVerifyFixture) => void;
    const pending = new Promise<typeof bridgeVerifyFixture>(resolve => {
      resolveVerification = resolve;
    });
    const store = createBridgeSetupFlowLogic().createStore({
      bridgeId: bridge.id,
      registrationFingerprint: "registration-v1",
      verificationFingerprint: "verification-v1",
    });
    store.trigger.verificationRequested({
      bridgeId: bridge.id,
      bridgeName: bridge.display_name,
      verify: () => pending,
    });
    await waitFor(() => expect(store.getSnapshot().context.phase).toBe("verifying"));

    store.trigger.verificationCleared();
    resolveVerification(bridgeVerifyFixture);
    await waitFor(() => {
      expect(store.getSnapshot().context.phase).toBe("idle");
      expect(store.getSnapshot().context.verification).toBeNull();
    });
  });

  it("Should route a rejected registration executor through recovery", async () => {
    const store = createBridgeSetupFlowLogic().createStore({
      bridgeId: bridge.id,
      registrationFingerprint: "registration-v1",
      verificationFingerprint: "verification-v1",
    });
    vi.mocked(toast.error).mockClear();

    store.trigger.registrationRequested({
      bridgeId: bridge.id,
      bridgeName: bridge.display_name,
      register: vi.fn().mockRejectedValue(new Error("registration unavailable")),
    });

    await waitFor(() => expect(store.getSnapshot().context.phase).toBe("recovery"));
    expect(toast.error).toHaveBeenCalledWith("registration unavailable");
  });
});
