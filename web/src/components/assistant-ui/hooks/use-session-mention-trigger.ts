import { useSelector, useStore } from "@xstate/store-react";

import type { SessionMentionTrigger } from "../session-mention-model";
import { activeMentionTrigger, sessionMentionTriggerLogic } from "../session-mention-trigger-store";

/**
 * The composer's live `@` occurrence, shared by the editor plugin that
 * observes it and the picker that serves it (they sit in different subtrees).
 */
export function useSessionMentionTrigger() {
  const store = useStore(sessionMentionTriggerLogic);
  const trigger = useSelector(store, snapshot => activeMentionTrigger(snapshot.context));
  return {
    mentionTrigger: trigger,
    observeMentionTrigger: (next: SessionMentionTrigger | null) =>
      store.trigger.observed({ trigger: next }),
    /** Closes the open picker for this `@`; false when nothing was open. */
    dismissMentionTrigger: (): boolean => {
      if (activeMentionTrigger(store.getSnapshot().context) === null) return false;
      store.trigger.dismissed();
      return true;
    },
  };
}
