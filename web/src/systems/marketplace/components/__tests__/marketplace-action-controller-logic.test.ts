// Suite: marketplace action controller logic
// Invariant: one dialog workflow is active and stale detail or trust completions cannot replace newer work.
// Boundary IN: pure marketplace dialog transitions.
// Boundary OUT: accepted Query, mutation, and authorization executors scheduled by the store.
import { describe, expect, it, vi } from "vitest";

import { marketplaceDetails, marketplaceListings } from "../../mocks";
import { marketplaceActionControllerLogic } from "../marketplace-action-controller-logic";

describe("marketplaceActionControllerLogic", () => {
  it("Should keep a trust dialog mounted and notify after its accepted install settles", async () => {
    const store = marketplaceActionControllerLogic.createStore();
    const entry = marketplaceListings.extension[0]!;
    let resolveInstall!: () => void;
    const install = new Promise<void>(resolve => {
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

    resolveInstall();
    await vi.waitFor(() => expect(store.getSnapshot().context.status).toBe("idle"));
    expect(notifySuccess).toHaveBeenCalledWith(entry);
  });

  it("Should fence stale detail and trust completions after a newer dialog request", () => {
    const store = marketplaceActionControllerLogic.createStore();
    const firstEntry = marketplaceListings.mcp[0]!;
    const secondEntry = marketplaceListings.mcp[1]!;
    const firstTrustEntry = marketplaceListings.extension[1]!;
    const secondTrustEntry = marketplaceListings.extension[0]!;

    store.trigger.detailRequested({
      describeFailure: String,
      kind: "mcp",
      load: () => new Promise<never>(() => undefined),
    });
    const firstDetail = store.getSnapshot().context;
    store.trigger.detailRequested({
      describeFailure: String,
      kind: "mcp",
      load: () => new Promise<never>(() => undefined),
    });
    const secondDetail = store.getSnapshot().context;
    if (firstDetail.status !== "detailLoading" || secondDetail.status !== "detailLoading") {
      throw new Error("The detail requests did not enter their loading phase.");
    }

    store.trigger.detailLoaded({
      detail: marketplaceDetails[firstEntry.entry_id]!,
      kind: "mcp",
      requestId: firstDetail.requestId,
    });
    expect(store.getSnapshot().context).toMatchObject({
      status: "detailLoading",
      requestId: secondDetail.requestId,
    });

    store.trigger.detailLoaded({
      detail: marketplaceDetails[secondEntry.entry_id]!,
      kind: "mcp",
      requestId: secondDetail.requestId,
    });
    expect(store.getSnapshot().context.status).toBe("mcpInstall");

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
