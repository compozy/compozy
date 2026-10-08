import { createStoreLogic } from "@xstate/store";

import type { AutomationEditorSeed } from "@/systems/automation";

interface AutomationCreateSeedState {
  consumedKey: string | null;
}

type AutomationCreateSeedEvents = {
  seedObserved: {
    activeWorkspaceId: string | null | undefined;
    /** `needs-project`: a Loop seed in Global — the Loop lives in a project. */
    consume: (seed: AutomationEditorSeed, outcome: "open" | "needs-project") => void;
    seed: AutomationEditorSeed | null;
    workspaceResolved: boolean;
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
      if (!event.workspaceResolved || context.consumedKey === seed.key) return;
      // Global (no project) is a resolved lens for a plain create; a Loop seed needs its project.
      const outcome =
        seed.loop !== undefined && !event.activeWorkspaceId ? "needs-project" : "open";
      enqueue.effect(() => event.consume(seed, outcome));
      return { consumedKey: seed.key };
    },
  },
});
