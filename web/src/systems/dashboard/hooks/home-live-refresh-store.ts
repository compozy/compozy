import { createStoreLogic } from "@xstate/store";

interface HomeLiveRefreshState {
  lastOverviewInvalidateAt: number;
  scope: string;
}

type HomeLiveRefreshEvents = {
  activityReceived: {
    at: number;
    invalidateOverview: () => void;
    invalidateTaskAggregates: () => void;
    lifecycle: boolean;
    minimumIntervalMs: number;
    scope: string;
  };
  scopeActivated: { scope: string };
};

function invoke(operation: () => void): void {
  try {
    operation();
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
        enqueue.effect(() => invoke(event.invalidateTaskAggregates));
        return { ...context, lastOverviewInvalidateAt: event.at };
      }
      if (event.at - context.lastOverviewInvalidateAt < event.minimumIntervalMs) return;
      enqueue.effect(() => invoke(event.invalidateOverview));
      return { ...context, lastOverviewInvalidateAt: event.at };
    },
  },
});
