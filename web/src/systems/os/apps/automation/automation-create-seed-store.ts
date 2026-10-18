import { createStoreLogic } from "@xstate/store";

import type { AutomationEditorSeed } from "@/systems/automation";

/** The deep link this page instance acted on. */
interface HandledSeed {
  key: string;
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
  /** The operator closed the editor this deep link opened. */
  operatorClosed: { strip: () => void };
  /** The editor saved: its navigation to the detail page replaces the URL. */
  saved: {};
};

/**
 * The `?create=` deep link stays in the URL while the editor it opened is
 * open, so a page that mounts late or remounts (cold load, window hydration,
 * a development double mount) opens it again. Only an explicit operator close
 * strips the params; an editor reset by anything else (a lens change, a
 * disposed lifecycle) reopens.
 */
export const automationCreateSeedLogic = createStoreLogic<
  AutomationCreateSeedState,
  AutomationCreateSeedEvents
>({
  context: { handled: null },
  on: {
    operatorClosed: (context, event, enqueue) => {
      if (!context.handled || context.handled.settled) return;
      enqueue.effect(event.strip);
      return { handled: { ...context.handled, settled: true } };
    },
    saved: context =>
      context.handled ? { handled: { ...context.handled, settled: true } } : undefined,
    seedObserved: (context, event, enqueue) => {
      const { seed, editorOpen, activeWorkspaceId } = event;
      if (seed === null) return context.handled === null ? undefined : { handled: null };
      if (!event.workspaceResolved || editorOpen) return;
      const handled = context.handled?.key === seed.key ? context.handled : null;
      if (seed.loop !== undefined && !activeWorkspaceId) {
        if (handled) return;
        enqueue.effect(() => {
          event.refuse();
          event.strip();
        });
        return { handled: { key: seed.key, settled: true } };
      }
      if (handled?.settled) return;
      enqueue.effect(() => event.open(seed));
      return { handled: { key: seed.key, settled: false } };
    },
  },
});
