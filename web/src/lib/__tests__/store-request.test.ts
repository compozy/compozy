import { describe, expect, it, vi } from "vitest";

import { awaitStoreRequest } from "../store-request";

// Suite: store request setup failure handling.
// Invariant: a request-setup failure rejects with its original error and leaves no store listener subscribed.
// Boundary IN: awaitStoreRequest's subscription registration and synchronous request initiation.
// Boundary OUT: caller-owned XState transition behavior, covered by its route and hook suites.
describe("awaitStoreRequest", () => {
  it("Should clean both subscriptions when request initiation throws", async () => {
    const requestFailure = new Error("request setup failed");
    const unsubscribeAccepted = vi.fn();
    const unsubscribeSettled = vi.fn();

    await expect(
      awaitStoreRequest<{ requestId: number }, { requestId: number }, void>({
        notAcceptedMessage: "Request was not accepted",
        request: () => {
          throw requestFailure;
        },
        resolveSettlement: () => undefined,
        subscribeAccepted: () => ({ unsubscribe: unsubscribeAccepted }),
        subscribeSettled: () => ({ unsubscribe: unsubscribeSettled }),
      })
    ).rejects.toBe(requestFailure);

    expect(unsubscribeAccepted).toHaveBeenCalledOnce();
    expect(unsubscribeSettled).toHaveBeenCalledOnce();
  });

  it("Should clean the first subscription when settlement subscription setup throws", async () => {
    const setupFailure = new Error("settlement subscription setup failed");
    const unsubscribeAccepted = vi.fn();

    await expect(
      awaitStoreRequest<{ requestId: number }, { requestId: number }, void>({
        notAcceptedMessage: "Request was not accepted",
        request: vi.fn(),
        resolveSettlement: () => undefined,
        subscribeAccepted: () => ({ unsubscribe: unsubscribeAccepted }),
        subscribeSettled: () => {
          throw setupFailure;
        },
      })
    ).rejects.toBe(setupFailure);

    expect(unsubscribeAccepted).toHaveBeenCalledOnce();
  });
});
