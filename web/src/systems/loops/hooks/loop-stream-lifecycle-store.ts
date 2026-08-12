import { createStoreLogic } from "@xstate/store";

export interface LoopStreamSubscription {
  generation: number;
  runId: string;
  workspaceId: string;
}

interface LoopStreamLifecycleState {
  activeSubscription: LoopStreamSubscription | null;
  generation: number;
}

type LoopStreamLifecycleEvents = {
  subscriptionClosed: { subscription: LoopStreamSubscription };
  subscriptionOpened: { subscription: LoopStreamSubscription };
};

export const loopStreamLifecycleLogic = createStoreLogic<
  LoopStreamLifecycleState,
  LoopStreamLifecycleEvents
>({
  context: { activeSubscription: null, generation: 0 },
  on: {
    subscriptionOpened: (_context, event) => ({
      activeSubscription: event.subscription,
      generation: event.subscription.generation,
    }),
    subscriptionClosed: (context, event) =>
      context.activeSubscription !== event.subscription
        ? undefined
        : { ...context, activeSubscription: null },
  },
});
