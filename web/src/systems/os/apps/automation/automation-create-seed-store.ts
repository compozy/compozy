import { createStoreLogic } from "@xstate/store";

interface AutomationCreateSeedState {
  consumedKey: string | null;
}

type AutomationCreateSeedEvents = {
  seedObserved: {
    activeWorkspaceId: string | null | undefined;
    consume: (key: string) => void;
    key: string | null;
  };
};

/** One-shot route intent consumption; URL callbacks enter through events. */
export const automationCreateSeedLogic = createStoreLogic<
  AutomationCreateSeedState,
  AutomationCreateSeedEvents
>({
  context: { consumedKey: null },
  on: {
    seedObserved: (context, event, enqueue) => {
      const key = event.key;
      if (key === null) {
        return context.consumedKey === null ? undefined : { consumedKey: null };
      }
      if (!event.activeWorkspaceId || context.consumedKey === key) return;
      enqueue.effect(() => event.consume(key));
      return { consumedKey: key };
    },
  },
});
