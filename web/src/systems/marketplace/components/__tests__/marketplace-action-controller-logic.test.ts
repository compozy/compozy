// Suite: marketplace action controller logic
// Invariant: one trust dialog workflow is active and stale completions cannot replace newer work.
// Boundary IN: pure marketplace dialog transitions.
// Boundary OUT: accepted Query, mutation, and authorization executors scheduled by the store.
import { describe, expect, it, vi } from "vitest";

import { marketplaceCatalogFixture } from "../../mocks";
import { marketplaceActionControllerLogic } from "../marketplace-action-controller-logic";

describe("marketplaceActionControllerLogic", () => {
  it("Should keep a trust dialog mounted and notify after its accepted install settles", async () => {
    const store = marketplaceActionControllerLogic.createStore();
    const entry = marketplaceCatalogFixture.items[0]!;
    let resolveInstall!: (notify: boolean) => void;
    const install = new Promise<boolean>(resolve => {
      resolveInstall = resolve;
    });
    const notifySuccess = vi.fn();
    store.trigger.extensionTrustRequested({ entry });
    store.trigger.extensionTrustConfirmed({
      describeFailure: String,
      execute: () => install,
      notifySuccess,
    });

    store.trigger.dialogDismissed();

    expect(store.getSnapshot().context).toMatchObject({
      status: "extensionTrustSubmitting",
      entry,
    });

    resolveInstall(true);
    await vi.waitFor(() => expect(store.getSnapshot().context.status).toBe("idle"));
    expect(notifySuccess).toHaveBeenCalledWith(entry);
  });

  it("Should close trust consent without a success notice when network consent takes over", async () => {
    const store = marketplaceActionControllerLogic.createStore();
    const entry = marketplaceCatalogFixture.items[1]!;
    const notifySuccess = vi.fn();
    store.trigger.extensionTrustRequested({ entry });
    store.trigger.extensionTrustConfirmed({
      describeFailure: String,
      execute: async () => false,
      notifySuccess,
    });

    await vi.waitFor(() => expect(store.getSnapshot().context.status).toBe("idle"));
    expect(notifySuccess).not.toHaveBeenCalled();
  });

  it("Should fence stale trust completions after a newer dialog request", () => {
    const store = marketplaceActionControllerLogic.createStore();
    const firstTrustEntry = marketplaceCatalogFixture.items[1]!;
    const secondTrustEntry = marketplaceCatalogFixture.items[0]!;

    store.trigger.extensionTrustRequested({ entry: firstTrustEntry });
    store.trigger.extensionTrustConfirmed({
      describeFailure: String,
      execute: vi.fn(() => new Promise<never>(() => undefined)),
      notifySuccess: vi.fn(),
    });
    const firstTrust = store.getSnapshot().context;
    if (firstTrust.status !== "extensionTrustSubmitting") {
      throw new Error("The first trust confirmation did not start.");
    }
    store.trigger.extensionTrustRequested({ entry: secondTrustEntry });
    store.trigger.extensionTrustConfirmed({
      describeFailure: String,
      execute: vi.fn(() => new Promise<never>(() => undefined)),
      notifySuccess: vi.fn(),
    });
    const secondTrust = store.getSnapshot().context;
    if (secondTrust.status !== "extensionTrustSubmitting") {
      throw new Error("The second trust confirmation did not start.");
    }

    store.trigger.extensionTrustSucceeded({
      notifySuccess: vi.fn(),
      requestId: firstTrust.requestId,
    });
    expect(store.getSnapshot().context).toMatchObject({
      status: "extensionTrustSubmitting",
      entry: secondTrustEntry,
      requestId: secondTrust.requestId,
    });
  });
});
