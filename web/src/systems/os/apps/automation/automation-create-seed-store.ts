import { createStoreLogic } from "@xstate/store";

import type { AutomationEditorSeed } from "@/systems/automation";

interface AutomationCreateSeedState {
  consumedKey: string | null;
}

type AutomationCreateSeedEvents = {
  seedObserved: {
    activeWorkspaceId: string | null | undefined;
    consume: (seed: AutomationEditorSeed) => void;
    seed: AutomationEditorSeed | null;
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
      const seed = event.seed;
      if (seed === null) {
        return context.consumedKey === null ? undefined : { consumedKey: null };
      }
      if (!event.activeWorkspaceId || context.consumedKey === seed.key) return;
      enqueue.effect(() => event.consume(seed));
      return { consumedKey: seed.key };
    },
  },
});
