import { createStoreLogic } from "@xstate/store";

interface AutomationEditIntentState {
  consumedKey: string | null;
}

type AutomationEditIntentEvents = {
  intentObserved: {
    /** `<entity id>:<edit param>` while the deep link is present; null once it is gone. */
    key: string | null;
    ready: boolean;
    consume: () => void;
  };
};

/**
 * One-shot `?edit=` consumption on a detail route: opens the editor once the
 * automation has loaded, then lets the caller strip the param. URL callbacks
 * enter through events.
 */
export const automationEditIntentLogic = createStoreLogic<
  AutomationEditIntentState,
  AutomationEditIntentEvents
>({
  context: { consumedKey: null },
  on: {
    intentObserved: (context, event, enqueue) => {
      if (event.key === null) {
        return context.consumedKey === null ? undefined : { consumedKey: null };
      }
      if (!event.ready || context.consumedKey === event.key) return;
      enqueue.effect(() => event.consume());
      return { consumedKey: event.key };
    },
  },
});
