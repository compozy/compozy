import { createStoreLogic, type EnqueueObject } from "@xstate/store";

interface HomeLiveRefreshState {
  lastOverviewInvalidateAt: number;
  scope: string;
  generation: number;
  refreshing: boolean;
  pending: boolean;
  retiredGenerations: readonly number[];
}

type InvalidateTaskAggregates = () => Promise<unknown>;

type HomeLiveRefreshEvents = {
  activityReceived: {
    at: number;
    invalidateOverview: () => Promise<unknown>;
    invalidateTaskAggregates: InvalidateTaskAggregates;
    lifecycle: boolean;
    minimumIntervalMs: number;
    scope: string;
  };
  scopeActivated: { scope: string };
  scopeDeactivated: { scope: string };
  refreshSettled: {
    scope: string;
    generation: number;
    invalidateTaskAggregates: InvalidateTaskAggregates;
  };
};

type RefreshEnqueue = EnqueueObject<HomeLiveRefreshState, never, HomeLiveRefreshEvents>;

async function invoke(operation: () => Promise<unknown>): Promise<void> {
  try {
    await operation();
  } catch (error) {
    console.error("Failed to invalidate a home live projection", error);
  }
}

function refreshAggregates(
  enqueue: RefreshEnqueue,
  scope: string,
  generation: number,
  invalidateTaskAggregates: InvalidateTaskAggregates
) {
  enqueue.effect(async ({ trigger }) => {
    await invoke(invalidateTaskAggregates);
    trigger.refreshSettled({ scope, generation, invalidateTaskAggregates });
  });
}

export const homeLiveRefreshLogic = createStoreLogic<HomeLiveRefreshState, HomeLiveRefreshEvents>({
  context: {
    lastOverviewInvalidateAt: 0,
    scope: "",
    generation: 0,
    refreshing: false,
    pending: false,
    retiredGenerations: [],
  },
  on: {
    scopeActivated: (context, event) =>
      context.scope === event.scope
        ? undefined
        : {
            ...context,
            lastOverviewInvalidateAt: 0,
            scope: event.scope,
            generation: context.generation + 1,
            refreshing: false,
            pending: false,
            retiredGenerations: context.pending
              ? [...context.retiredGenerations, context.generation]
              : context.retiredGenerations,
          },
    scopeDeactivated: (context, event) => {
      if (event.scope !== context.scope) return;
      return {
        ...context,
        scope: "",
        generation: context.generation + 1,
        refreshing: false,
        pending: false,
        retiredGenerations: context.pending
          ? [...context.retiredGenerations, context.generation]
          : context.retiredGenerations,
      };
    },
    refreshSettled: (context, event, enqueue) => {
      if (context.retiredGenerations.includes(event.generation)) {
        // Wait until the old response can no longer overwrite the owed invalidation.
        enqueue.effect(() => invoke(event.invalidateTaskAggregates));
        return {
          ...context,
          retiredGenerations: context.retiredGenerations.filter(
            value => value !== event.generation
          ),
        };
      }
      if (event.scope !== context.scope || event.generation !== context.generation) return;
      if (context.pending) {
        refreshAggregates(enqueue, event.scope, event.generation, event.invalidateTaskAggregates);
      }
      return { ...context, pending: false, refreshing: context.pending };
    },
    activityReceived: (context, event, enqueue) => {
      if (event.scope !== context.scope) return;
      if (event.lifecycle) {
        if (!context.refreshing) {
          refreshAggregates(
            enqueue,
            context.scope,
            context.generation,
            event.invalidateTaskAggregates
          );
        }
        return {
          ...context,
          lastOverviewInvalidateAt: event.at,
          refreshing: true,
          pending: context.refreshing,
        };
      }
      if (event.at - context.lastOverviewInvalidateAt < event.minimumIntervalMs) return;
      enqueue.effect(async () => {
        await invoke(event.invalidateOverview);
      });
      return { ...context, lastOverviewInvalidateAt: event.at };
    },
  },
});
