import { waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import { toast } from "sonner";

import type {
  BridgeSummary,
  SendBridgeTestResponse,
  TestBridgeDeliveryResponse,
} from "@/systems/bridges";

import { bridgeDeliveryTestsLogic } from "../bridge-delivery-tests-store";

const bridge = {
  created_at: "2026-07-25T12:00:00Z",
  delivery_defaults: {},
  id: "bridge-alpha",
  display_name: "Alpha bridge",
  dm_policy: "open",
  enabled: true,
  extension_name: "ext-alpha",
  notification_suppress: false,
  platform: "test",
  provider_config: {},
  routing_policy: { include_group: true, include_peer: true, include_thread: true },
  scope: "workspace",
  status: "ready",
  updated_at: "2026-07-25T12:00:00Z",
  workspace_id: "workspace-alpha",
} satisfies BridgeSummary;

const dryRunResult: TestBridgeDeliveryResponse = {
  delivery_target: { bridge_instance_id: bridge.id },
  status: "resolved",
};
const sendTestResult: SendBridgeTestResponse = {
  bridge_instance_id: bridge.id,
  delivery_id: "delivery-alpha",
  delivery_target: { bridge_instance_id: bridge.id },
  status: "delivered",
};

const pendingDryRun = () => new Promise<TestBridgeDeliveryResponse>(() => undefined);
const pendingSendTest = () => new Promise<SendBridgeTestResponse>(() => undefined);

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(next => {
    resolve = next;
  });
  return { promise, resolve };
}

describe("bridge delivery tests store", () => {
  it("Should keep dry-run and send-test phases independent", () => {
    const store = bridgeDeliveryTestsLogic.createStore();
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, { type: "reset", bridge });
    [snapshot] = store.transition(snapshot, {
      type: "dryRunSubmitted",
      bridge,
      execute: pendingDryRun,
    });

    expect(snapshot.context.dryRun.phase).toBe("submitting");
    expect(snapshot.context.sendTest.phase).toBe("idle");
  });

  it("Should settle a real in-flight result after the next draft changes", async () => {
    const store = bridgeDeliveryTestsLogic.createStore();
    const pending = deferred<TestBridgeDeliveryResponse>();
    vi.mocked(toast.success).mockClear();
    store.trigger.reset({ bridge });
    store.trigger.dryRunSubmitted({ bridge, execute: () => pending.promise });
    store.trigger.draftChanged({
      draft: { ...store.getSnapshot().context.draft, message: "replacement" },
    });
    pending.resolve(dryRunResult);

    await waitFor(() => {
      expect(store.getSnapshot().context.dryRun.phase).toBe("resolved");
    });
    expect(store.getSnapshot().context.draft.message).toBe("replacement");
    expect(toast.success).toHaveBeenCalledWith("Resolved delivery target for Alpha bridge.");
  });

  it("Should retain the last delivery result while editing the next draft", async () => {
    const store = bridgeDeliveryTestsLogic.createStore();
    store.trigger.reset({ bridge });
    store.trigger.dryRunSubmitted({
      bridge,
      execute: vi.fn().mockResolvedValue(dryRunResult),
    });
    await waitFor(() => expect(store.getSnapshot().context.dryRun.phase).toBe("resolved"));

    store.trigger.draftChanged({
      draft: { ...store.getSnapshot().context.draft, message: "next message" },
    });

    expect(store.getSnapshot().context.dryRun).toMatchObject({
      phase: "resolved",
      result: dryRunResult,
    });
  });

  it("Should disallow a real send without an enabled bridge and message", () => {
    const store = bridgeDeliveryTestsLogic.createStore();
    store.trigger.reset({ bridge: { ...bridge, enabled: false } });
    expect(
      store.can.sendTestSubmitted({
        bridge: { ...bridge, enabled: false },
        execute: pendingSendTest,
      })
    ).toBe(false);

    store.trigger.reset({ bridge });
    expect(store.can.sendTestSubmitted({ bridge, execute: pendingSendTest })).toBe(false);

    store.trigger.draftChanged({
      draft: { ...store.getSnapshot().context.draft, message: "Hello" },
    });
    expect(store.can.sendTestSubmitted({ bridge, execute: pendingSendTest })).toBe(true);
  });

  it("Should accept the active real send-test result", async () => {
    const store = bridgeDeliveryTestsLogic.createStore();
    store.trigger.reset({ bridge });
    store.trigger.draftChanged({
      draft: { ...store.getSnapshot().context.draft, message: "Hello" },
    });
    store.trigger.sendTestSubmitted({
      bridge,
      execute: vi.fn().mockResolvedValue(sendTestResult),
    });

    await waitFor(() => expect(store.getSnapshot().context.sendTest.phase).toBe("resolved"));
    expect(store.getSnapshot().context.sendTest).toMatchObject({ result: sendTestResult });
  });

  it("Should submit a real send with the bridge and draft from its event", async () => {
    const store = bridgeDeliveryTestsLogic.createStore();
    const execute = vi.fn().mockResolvedValue(sendTestResult);
    store.trigger.reset({ bridge: { ...bridge, enabled: false } });
    store.trigger.draftChanged({
      draft: { ...store.getSnapshot().context.draft, message: "Hello" },
    });
    store.trigger.sendTestSubmitted({ bridge, execute });

    await waitFor(() =>
      expect(execute).toHaveBeenCalledWith(bridge, expect.objectContaining({ message: "Hello" }))
    );
  });

  it("Should surface a rejected real dry-run executor", async () => {
    const store = bridgeDeliveryTestsLogic.createStore();
    vi.mocked(toast.error).mockClear();
    store.trigger.reset({ bridge });
    store.trigger.dryRunSubmitted({
      bridge,
      execute: vi.fn().mockRejectedValue(new Error("target unavailable")),
    });

    await waitFor(() => expect(store.getSnapshot().context.dryRun.phase).toBe("failed"));
    expect(toast.error).toHaveBeenCalledWith("target unavailable");
  });
});
