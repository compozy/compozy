import type { Unstable_TriggerItem } from "@assistant-ui/react";
import { useSelector, useStore } from "@xstate/store-react";

import type { SessionMentionTrigger } from "../session-mention-model";
import { activeMentionTrigger, sessionMentionTriggerLogic } from "../session-mention-trigger-store";
import { useMentionWorkspaceId } from "./use-mention-workspace-id";

/**
 * The composer's live `@` occurrence and recents, shared by the editor plugin
 * that observes the caret, the directive plugin that reports picks, and the
 * picker that serves them (they sit in different subtrees).
 */
export function useSessionMentionTrigger() {
  const workspaceId = useMentionWorkspaceId();
  const store = useStore(sessionMentionTriggerLogic, { workspaceId });
  const trigger = useSelector(store, snapshot => activeMentionTrigger(snapshot.context));
  const recents = useSelector(store, snapshot => snapshot.context.recents);
  return {
    mentionTrigger: trigger,
    mentionRecents: recents,
    observeMentionTrigger: (next: SessionMentionTrigger | null) =>
      store.trigger.observed({ trigger: next }),
    /** Closes the open picker for this `@`; false when nothing was open. */
    dismissMentionTrigger: (): boolean => {
      if (activeMentionTrigger(store.getSnapshot().context) === null) return false;
      store.trigger.dismissed();
      return true;
    },
    /** A picked file or folder becomes this workspace's most recent mention. */
    recordMentionPick: (item: Unstable_TriggerItem) => {
      const kind = item.metadata?.kind;
      const target = item.metadata?.target;
      if ((kind !== "file" && kind !== "folder") || typeof target !== "string") return;
      store.trigger.picked({ recent: { path: target, kind } });
    },
  };
}
