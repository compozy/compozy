import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import { useSelector } from "@xstate/store-react";

import { normalizeComputerId, type ComputerId } from "@/lib/computer-scope";

import { composerDraftAttachmentFiles } from "../lib/session-composer-attachment-drafts";
import { sessionRowKey } from "../lib/session-row-ref";
import { sessionStore, type SessionGoalFeedback } from "../stores/session-store";

function storeKey(sessionId: string, computerId?: ComputerId): string {
  return sessionRowKey({ computerId: normalizeComputerId(computerId), sessionId });
}

export function useSessionComposerDraft(sessionId: string, computerId?: ComputerId): string {
  const key = storeKey(sessionId, computerId);
  return useSelector(sessionStore, snapshot => snapshot.context.drafts[key] ?? "");
}

/** Whether the session's composer holds unsent text; flips only when the draft empties or fills. */
export function useSessionHasDraft(sessionId: string, computerId?: ComputerId): boolean {
  const key = storeKey(sessionId, computerId);
  return useSelector(sessionStore, snapshot => (snapshot.context.drafts[key] ?? "").trim() !== "");
}

/** Unsent composer input at this moment: draft text or attached files not yet sent. */
export function sessionHasUnsentInput(sessionId: string, computerId?: ComputerId): boolean {
  const key = storeKey(sessionId, computerId);
  const draft = sessionStore.getSnapshot().context.drafts[key] ?? "";
  return draft.trim() !== "" || composerDraftAttachmentFiles(key).length > 0;
}

export function useSessionGoalFeedback(
  sessionId: string,
  computerId?: ComputerId
): SessionGoalFeedback | undefined {
  const key = storeKey(sessionId, useCurrentComputerId(computerId));
  return useSelector(sessionStore, snapshot => snapshot.context.goalFeedback[key]);
}

/** The refusal belongs to the recovered draft until it is sent or discarded. */
export function useSessionPromptRefusal(
  sessionId: string,
  computerId?: ComputerId
): string | undefined {
  const key = storeKey(sessionId, useCurrentComputerId(computerId));
  return useSelector(sessionStore, snapshot => snapshot.context.promptRefusals[key]);
}
