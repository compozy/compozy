import { createStoreLogic, type EnqueueObject } from "@xstate/store";

export type MarketplaceInstalledCardAction = "remove";

export type MarketplaceInstalledCardConfirmState =
  | { phase: "closed"; requestId: number }
  | { phase: "confirming"; requestId: number; action: MarketplaceInstalledCardAction }
  | { phase: "pending"; requestId: number; action: MarketplaceInstalledCardAction }
  | { phase: "failed"; requestId: number; action: MarketplaceInstalledCardAction; error: string };

type Events = {
  confirmationOpened: { action: MarketplaceInstalledCardAction };
  confirmationCancelled: {};
  confirmationRequested: {
    execute: (action: MarketplaceInstalledCardAction) => Promise<void>;
  };
  confirmationSucceeded: { requestId: number };
  confirmationFailed: { requestId: number; error: string };
};

type Enqueue = EnqueueObject<MarketplaceInstalledCardConfirmState, never, Events>;

export function createMarketplaceInstalledCardConfirmLogic() {
  return createStoreLogic<MarketplaceInstalledCardConfirmState, Events>({
    context: { phase: "closed", requestId: 0 },
    on: {
      confirmationOpened: (context, event) => ({
        phase: "confirming",
        requestId: context.requestId + 1,
        action: event.action,
      }),
      confirmationCancelled: context => ({ phase: "closed", requestId: context.requestId + 1 }),
      confirmationRequested: (context, event, enqueue) => {
        if (context.phase !== "confirming" && context.phase !== "failed") return;
        const requestId = context.requestId + 1;
        enqueueExecution(enqueue, context.action, requestId, event.execute);
        return { phase: "pending", requestId, action: context.action };
      },
      confirmationSucceeded: (context, event) => {
        if (context.phase !== "pending" || context.requestId !== event.requestId) return;
        return { phase: "closed", requestId: context.requestId };
      },
      confirmationFailed: (context, event) => {
        if (context.phase !== "pending" || context.requestId !== event.requestId) return;
        return { ...context, phase: "failed", error: event.error };
      },
    },
  });
}

function enqueueExecution(
  enqueue: Enqueue,
  action: MarketplaceInstalledCardAction,
  requestId: number,
  execute: (action: MarketplaceInstalledCardAction) => Promise<void>
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await execute(action);
      trigger.confirmationSucceeded({ requestId });
    } catch (error) {
      trigger.confirmationFailed({ requestId, error: errorMessage(error) });
    }
  });
}

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : "Action failed";
}
