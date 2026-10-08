import { createStoreLogic } from "@xstate/store";

import type { AutomationEditorSeed } from "@/systems/automation";

/** The deep link this page instance acted on, and the lens it opened the editor under. */
interface HandledSeed {
  key: string;
  workspaceId: string | null | undefined;
  /** The URL no longer needs cleaning: it was stripped, or a save navigated away. */
  settled: boolean;
}

interface AutomationCreateSeedState {
  handled: HandledSeed | null;
}

type AutomationCreateSeedEvents = {
  seedObserved: {
    activeWorkspaceId: string | null | undefined;
    editorOpen: boolean;
    seed: AutomationEditorSeed | null;
    workspaceResolved: boolean;
    open: (seed: AutomationEditorSeed) => void;
    /** A Loop seed in Global: the Loop lives in a project. */
    refuse: () => void;
    strip: () => void;
  };
  /** The editor saved: its navigation to the detail page replaces the URL. */
  saved: {};
};

/**
 * The `?create=` deep link stays in the URL while the editor it opened is
 * open, so a page that mounts late or remounts (cold load, window hydration)
 * opens it again. Only a close the operator made — the editor closed under the
 * same lens it was opened in — strips the params. A lens change resets the
 * editor without the operator, so it reopens.
 */
export const automationCreateSeedLogic = createStoreLogic<
  AutomationCreateSeedState,
  AutomationCreateSeedEvents
>({
  context: { handled: null },
  on: {
    saved: context =>
      context.handled ? { handled: { ...context.handled, settled: true } } : undefined,
    seedObserved: (context, event, enqueue) => {
      const { seed, editorOpen, activeWorkspaceId } = event;
      if (seed === null) return context.handled === null ? undefined : { handled: null };
      if (!event.workspaceResolved || editorOpen) return;
      const handled = context.handled;
      const sameSeed = handled?.key === seed.key;
      if (seed.loop !== undefined && !activeWorkspaceId) {
        if (sameSeed) return;
        enqueue.effect(() => {
          event.refuse();
          event.strip();
        });
        return { handled: { key: seed.key, workspaceId: activeWorkspaceId, settled: true } };
      }
      if (sameSeed && handled.workspaceId === activeWorkspaceId) {
        if (handled.settled) return;
        enqueue.effect(event.strip);
        return { handled: { ...handled, settled: true } };
      }
      enqueue.effect(() => event.open(seed));
      return { handled: { key: seed.key, workspaceId: activeWorkspaceId, settled: false } };
    },
  },
});
