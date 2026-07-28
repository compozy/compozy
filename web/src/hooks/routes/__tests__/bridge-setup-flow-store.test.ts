import { waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn(), warning: vi.fn() },
}));

import { toast } from "sonner";

import { bridgeVerifyFixture } from "@/systems/bridges/mocks";
import { createBridgeSetupFlowLogic } from "../bridge-setup-flow-store";

describe("bridge setup flow store", () => {
  it("Should keep verification pending and discard only stale evidence after facts change", () => {
    const store = createBridgeSetupFlowLogic().createStore({
      bridgeId: "bridge-alpha",
      registrationFingerprint: "registration-v1",
      verificationFingerprint: "verification-v1",
    });
    const verify = vi.fn(() => new Promise<never>(() => undefined));
    vi.mocked(toast.success).mockClear();

    store.trigger.verificationRequested({
      bridgeId: "bridge-alpha",
      bridgeName: "Alpha",
      verify,
    });
    const attempt = store.getSnapshot().context.attempt;
    store.trigger.factsChanged({
      bridgeId: "bridge-alpha",
      registrationFingerprint: "registration-v1",
      verificationFingerprint: "verification-v2",
    });

    expect(store.getSnapshot().context.phase).toBe("verifying");
    expect(
      store.can.verificationRequested({
        bridgeId: "bridge-alpha",
        bridgeName: "Alpha",
        verify,
      })
    ).toBe(false);

    store.trigger.verificationResolved({
      attempt,
      bridgeId: "bridge-alpha",
      bridgeName: "Alpha",
      result: { ...bridgeVerifyFixture, bridge_instance_id: "bridge-alpha" },
    });

    expect(store.getSnapshot().context.phase).toBe("idle");
    expect(store.getSnapshot().context.verification).toBeNull();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("Should ignore a verification result after setup evidence resets", async () => {
    let resolveVerification!: (result: typeof bridgeVerifyFixture) => void;
    const pending = new Promise<typeof bridgeVerifyFixture>(resolve => {
      resolveVerification = resolve;
    });
    const store = createBridgeSetupFlowLogic().createStore({
      bridgeId: "bridge-alpha",
      registrationFingerprint: "registration-v1",
      verificationFingerprint: "verification-v1",
    });
    store.trigger.verificationRequested({
      bridgeId: "bridge-alpha",
      bridgeName: "Alpha",
      verify: () => pending,
    });
    await waitFor(() => expect(store.getSnapshot().context.phase).toBe("verifying"));

    store.trigger.verificationCleared();
    resolveVerification({ ...bridgeVerifyFixture, bridge_instance_id: "bridge-alpha" });
    await waitFor(() => {
      expect(store.getSnapshot().context.phase).toBe("idle");
      expect(store.getSnapshot().context.verification).toBeNull();
    });
  });

  it("Should route a rejected registration executor through recovery", async () => {
    const store = createBridgeSetupFlowLogic().createStore({
      bridgeId: "bridge-alpha",
      registrationFingerprint: "registration-v1",
      verificationFingerprint: "verification-v1",
    });
    vi.mocked(toast.error).mockClear();

    store.trigger.registrationRequested({
      bridgeId: "bridge-alpha",
      bridgeName: "Alpha",
      register: vi.fn().mockRejectedValue(new Error("registration unavailable")),
    });

    await waitFor(() => expect(store.getSnapshot().context.phase).toBe("recovery"));
    expect(toast.error).toHaveBeenCalledWith("registration unavailable");
  });
});
