import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import type { MarketplaceEntryResponse, MarketplaceListing } from "../types";

export type MarketplaceActionControllerPhase =
  | { status: "idle"; nextRequestId: number }
  | { status: "detailLoading"; nextRequestId: number; requestId: number }
  | { status: "mcpInstall"; nextRequestId: number; detail: MarketplaceEntryResponse }
  | { status: "bundleActivation"; nextRequestId: number; detail: MarketplaceEntryResponse }
  | {
      status: "extensionTrust";
      nextRequestId: number;
      entry: MarketplaceListing;
      error: string | null;
    }
  | {
      status: "extensionTrustSubmitting";
      nextRequestId: number;
      entry: MarketplaceListing;
      requestId: number;
    };

type MarketplaceActionControllerEventPayloadMap = {
  detailFailed: { error: string; requestId: number };
  detailLoaded: {
    detail: MarketplaceEntryResponse;
    kind: "bundle" | "mcp";
    requestId: number;
  };
  detailRequested: {
    describeFailure: (error: unknown) => string;
    kind: "bundle" | "mcp";
    load: () => Promise<MarketplaceEntryResponse>;
  };
  dialogDismissed: {};
  extensionTrustConfirmed: {
    describeFailure: (error: unknown) => string;
    execute: (entry: MarketplaceListing) => Promise<void>;
    notifySuccess: (entry: MarketplaceListing) => void;
  };
  extensionTrustFailed: { error: string; requestId: number };
  extensionTrustRequested: { entry: MarketplaceListing };
  extensionTrustSucceeded: {
    notifySuccess: (entry: MarketplaceListing) => void;
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
    detailFailed: (context, event, enqueue) => {
      if (context.status !== "detailLoading" || context.requestId !== event.requestId) return;
      enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
      return idle(context.nextRequestId);
    },
    detailLoaded: (context, event) => {
      if (context.status !== "detailLoading" || context.requestId !== event.requestId) return;
      return event.kind === "mcp"
        ? { detail: event.detail, nextRequestId: context.nextRequestId, status: "mcpInstall" }
        : {
            detail: event.detail,
            nextRequestId: context.nextRequestId,
            status: "bundleActivation",
          };
    },
    detailRequested: (context, event, enqueue) => {
      const requestId = context.nextRequestId + 1;
      enqueueDetail(event, enqueue, requestId);
      return { nextRequestId: requestId, requestId, status: "detailLoading" };
    },
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

function enqueueDetail(
  event: MarketplaceActionControllerEventPayloadMap["detailRequested"],
  enqueue: MarketplaceActionControllerEnqueue,
  requestId: number
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      trigger.detailLoaded({ detail: await event.load(), kind: event.kind, requestId });
    } catch (error) {
      trigger.detailFailed({ error: event.describeFailure(error), requestId });
    }
  });
}

function enqueueExtensionTrust(
  entry: MarketplaceListing,
  event: MarketplaceActionControllerEventPayloadMap["extensionTrustConfirmed"],
  enqueue: MarketplaceActionControllerEnqueue,
  requestId: number
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await event.execute(entry);
      trigger.extensionTrustSucceeded({ notifySuccess: event.notifySuccess, requestId });
    } catch (error) {
      trigger.extensionTrustFailed({ error: event.describeFailure(error), requestId });
    }
  });
}
