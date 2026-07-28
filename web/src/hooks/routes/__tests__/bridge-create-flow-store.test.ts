import { describe, expect, it, vi } from "vitest";

import { createBridgeCreateDraft } from "@/systems/bridges";
import {
  bridgeProvidersFixture,
  bridgesListFixture,
  createBridgeFixture,
} from "@/systems/bridges/mocks";

import { createBridgeCreateFlowLogic } from "../bridge-create-flow-store";

const bridge = bridgesListFixture.bridges[0];
const provider = bridgeProvidersFixture[0];

if (!bridge || !provider) {
  throw new Error("Bridge creation flow fixtures require one bridge and one provider.");
}

describe("bridge create flow store", () => {
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

  it("Should not restore secret recovery after completed binding fails only to navigate", () => {
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
    [snapshot] = store.transition(snapshot, {
      type: "secretRetrySubmitted",
      bindSecrets: async () => ({ bound: [], clearedSlotNames: [], failures: {} }),
      navigate: async () => undefined,
    });
    [snapshot] = store.transition(snapshot, {
      type: "secretRetryCompleted",
      attempt: snapshot.context.attempt,
      bindOutcome: { bound: ["api_key"], clearedSlotNames: ["api_key"], failures: {} },
      navigate: async () => undefined,
    });
    [snapshot] = store.transition(snapshot, {
      type: "navigationFailed",
      attempt: snapshot.context.attempt,
      error: "router unavailable",
    });

    expect(snapshot.context.phase).toBe("closed");
  });
});
