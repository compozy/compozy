import { useSelector } from "@xstate/store-react";

import { composerDraftAttachmentFiles } from "../lib/session-composer-attachment-drafts";
import { sessionStore, type SessionGoalFeedback } from "../stores/session-store";

export function useSessionComposerDraft(sessionId: string): string {
  return useSelector(sessionStore, snapshot => snapshot.context.drafts[sessionId] ?? "");
}

/** Whether the session's composer holds unsent text; flips only when the draft empties or fills. */
export function useSessionHasDraft(sessionId: string): boolean {
  return useSelector(
    sessionStore,
    snapshot => (snapshot.context.drafts[sessionId] ?? "").trim() !== ""
  );
}

/** Unsent composer input at this moment: draft text or attached files not yet sent. */
export function sessionHasUnsentInput(sessionId: string): boolean {
  const draft = sessionStore.getSnapshot().context.drafts[sessionId] ?? "";
  return draft.trim() !== "" || composerDraftAttachmentFiles(sessionId).length > 0;
}

export function useSessionGoalFeedback(sessionId: string): SessionGoalFeedback | undefined {
  return useSelector(sessionStore, snapshot => snapshot.context.goalFeedback[sessionId]);
}
