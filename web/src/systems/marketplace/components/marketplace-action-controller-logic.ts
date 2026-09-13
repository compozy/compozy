import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import type { MarketplaceCatalogListing } from "../types";

/** Explicit-consent gate for an `allowed_unverified` catalog entry, in daemon order. */
export type MarketplaceActionControllerPhase =
  | { status: "idle"; nextRequestId: number }
  | {
      status: "extensionTrust";
      nextRequestId: number;
      entry: MarketplaceCatalogListing;
      error: string | null;
    }
  | {
      status: "extensionTrustSubmitting";
      nextRequestId: number;
      entry: MarketplaceCatalogListing;
      requestId: number;
    };

type MarketplaceActionControllerEventPayloadMap = {
  dialogDismissed: {};
  extensionTrustConfirmed: {
    describeFailure: (error: unknown) => string;
    execute: (entry: MarketplaceCatalogListing) => Promise<boolean>;
    notifySuccess: (entry: MarketplaceCatalogListing) => void;
  };
  extensionTrustFailed: { error: string; requestId: number };
  extensionTrustRequested: { entry: MarketplaceCatalogListing };
  extensionTrustSucceeded: {
    notifySuccess: (entry: MarketplaceCatalogListing) => void;
    requestId: number;
  };
};

type MarketplaceActionControllerEnqueue = EnqueueObject<
  MarketplaceActionControllerPhase,
  never,
  MarketplaceActionControllerEventPayloadMap
>;

function idle(nextRequestId: number): MarketplaceActionControllerPhase {
  return { nextRequestId, status: "idle" };
}

export const marketplaceActionControllerLogic = createStoreLogic<
  MarketplaceActionControllerPhase,
  MarketplaceActionControllerEventPayloadMap
>({
  context: idle(0),
  on: {
    dialogDismissed: context =>
      context.status === "extensionTrustSubmitting" ? undefined : idle(context.nextRequestId),
    extensionTrustConfirmed: (context, event, enqueue) => {
      if (context.status !== "extensionTrust") return;
      const requestId = context.nextRequestId + 1;
      enqueueExtensionTrust(context.entry, event, enqueue, requestId);
      return {
        entry: context.entry,
        nextRequestId: requestId,
        requestId,
        status: "extensionTrustSubmitting",
      };
    },
    extensionTrustFailed: (context, event) => {
      if (context.status !== "extensionTrustSubmitting" || context.requestId !== event.requestId) {
        return;
      }
      return {
        entry: context.entry,
        error: event.error,
        nextRequestId: context.nextRequestId,
        status: "extensionTrust",
      };
    },
    extensionTrustRequested: (context, event) => ({
      entry: event.entry,
      error: null,
      nextRequestId: context.nextRequestId,
      status: "extensionTrust",
    }),
    extensionTrustSucceeded: (context, event, enqueue) => {
      if (context.status !== "extensionTrustSubmitting" || context.requestId !== event.requestId) {
        return;
      }
      enqueue.effect(() => event.notifySuccess(context.entry));
      return idle(context.nextRequestId);
    },
  },
});

function enqueueExtensionTrust(
  entry: MarketplaceCatalogListing,
  event: MarketplaceActionControllerEventPayloadMap["extensionTrustConfirmed"],
  enqueue: MarketplaceActionControllerEnqueue,
  requestId: number
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      const notify = await event.execute(entry);
      trigger.extensionTrustSucceeded({
        notifySuccess: notify ? event.notifySuccess : () => undefined,
        requestId,
      });
    } catch (error) {
      trigger.extensionTrustFailed({ error: event.describeFailure(error), requestId });
    }
  });
}
