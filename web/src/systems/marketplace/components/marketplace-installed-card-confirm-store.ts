import { createStoreLogic, type EnqueueObject } from "@xstate/store";

export type MarketplaceInstalledCardConfirmState =
  | { phase: "closed"; requestId: number }
  | { phase: "confirming"; requestId: number }
  | { phase: "pending"; requestId: number }
  | { phase: "failed"; requestId: number; error: string };

type Events = {
  confirmationOpened: {};
  confirmationCancelled: {};
  confirmationRequested: {
    execute: () => Promise<void>;
  };
  confirmationSucceeded: { requestId: number };
  confirmationFailed: { requestId: number; error: string };
};

type Enqueue = EnqueueObject<MarketplaceInstalledCardConfirmState, never, Events>;

export function createMarketplaceInstalledCardConfirmLogic() {
  return createStoreLogic<MarketplaceInstalledCardConfirmState, Events>({
    context: { phase: "closed", requestId: 0 },
    on: {
      confirmationOpened: context => ({
        phase: "confirming",
        requestId: context.requestId + 1,
      }),
      confirmationCancelled: context => ({ phase: "closed", requestId: context.requestId + 1 }),
      confirmationRequested: (context, event, enqueue) => {
        if (context.phase !== "confirming" && context.phase !== "failed") return;
        const requestId = context.requestId + 1;
        enqueueExecution(enqueue, requestId, event.execute);
        return { phase: "pending", requestId };
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

function enqueueExecution(enqueue: Enqueue, requestId: number, execute: () => Promise<void>) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await execute();
      trigger.confirmationSucceeded({ requestId });
    } catch (error) {
      trigger.confirmationFailed({ requestId, error: errorMessage(error) });
    }
  });
}

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : "Action failed";
}
