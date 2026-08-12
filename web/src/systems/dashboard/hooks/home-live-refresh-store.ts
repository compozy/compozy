import { createStoreLogic } from "@xstate/store";

interface HomeLiveRefreshState {
  lastOverviewInvalidateAt: number;
  scope: string;
}

type HomeLiveRefreshEvents = {
  activityReceived: {
    at: number;
    invalidateOverview: () => Promise<unknown>;
    invalidateTaskAggregates: () => Promise<unknown>;
    lifecycle: boolean;
    minimumIntervalMs: number;
    scope: string;
  };
  scopeActivated: { scope: string };
};

async function invoke(operation: () => Promise<unknown>): Promise<void> {
  try {
    await operation();
  } catch (error) {
    console.error("Failed to invalidate a home live projection", error);
  }
}

export const homeLiveRefreshLogic = createStoreLogic<HomeLiveRefreshState, HomeLiveRefreshEvents>({
  context: { lastOverviewInvalidateAt: 0, scope: "" },
  on: {
    scopeActivated: (context, event) =>
      context.scope === event.scope
        ? undefined
        : { lastOverviewInvalidateAt: 0, scope: event.scope },
    activityReceived: (context, event, enqueue) => {
      if (event.scope !== context.scope) return;
      if (event.lifecycle) {
        enqueue.effect(async () => {
          await invoke(event.invalidateTaskAggregates);
        });
        return { ...context, lastOverviewInvalidateAt: event.at };
      }
      if (event.at - context.lastOverviewInvalidateAt < event.minimumIntervalMs) return;
      enqueue.effect(async () => {
        await invoke(event.invalidateOverview);
      });
      return { ...context, lastOverviewInvalidateAt: event.at };
    },
  },
});
