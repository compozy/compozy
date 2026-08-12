import { createStoreLogic } from "@xstate/store";

interface AutomationCreateSeedState {
  consumedLoop: string | null;
}

type AutomationCreateSeedEvents = {
  seedObserved: {
    activeWorkspaceId: string | null | undefined;
    consume: (loop: string) => void;
    loop: string | null;
  };
};

/** One-shot route intent consumption; URL callbacks enter through events. */
export const automationCreateSeedLogic = createStoreLogic<
  AutomationCreateSeedState,
  AutomationCreateSeedEvents
>({
  context: { consumedLoop: null },
  on: {
    seedObserved: (context, event, enqueue) => {
      const loop = event.loop;
      if (loop === null) {
        return context.consumedLoop === null ? undefined : { consumedLoop: null };
      }
      if (!event.activeWorkspaceId || context.consumedLoop === loop) return;
      enqueue.effect(() => event.consume(loop));
      return { consumedLoop: loop };
    },
  },
});
