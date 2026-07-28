import { useSelector } from "@xstate/store-react";

import { sessionStore, type SessionGoalFeedback } from "../stores/session-store";

export function useSessionComposerDraft(sessionId: string): string {
  return useSelector(sessionStore, snapshot => snapshot.context.drafts[sessionId] ?? "");
}

export function useSessionGoalFeedback(sessionId: string): SessionGoalFeedback | undefined {
  return useSelector(sessionStore, snapshot => snapshot.context.goalFeedback[sessionId]);
}
