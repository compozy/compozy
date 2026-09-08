import { useAui } from "@assistant-ui/react";
import { useQuery } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import type { SessionPromptDispatchStore } from "@/components/assistant-ui/session-prompt-dispatch-store";
import { derivePendingClarifyRequestIds } from "../lib/clarify-event";
import {
  deriveDecidedPermissionRequestIds,
  derivePendingPermissions,
} from "../lib/pending-permissions";
import { sessionDetailOptions } from "../lib/query-options";
import { sessionStore } from "../stores/session-store";
import { useMergedSessionRuntimeTranscript } from "./use-merged-session-runtime-transcript";
import { useSessionClarifications } from "./use-session-clarifications";
import { useSessionExpiredInteractions } from "./use-session-expired-interactions";
import { useSessionInputs } from "./use-session-inputs";
import { useSessionResolvedInteractions } from "./use-session-resolved-interactions";
import type { SessionStreamEventSourceFactory } from "./use-session-live-tail";

/**
 * While a prompt POST streams, the transcript SSE stays closed (HTTP/1.1 pool)
 * and the POST owns active-turn data; a bounded 1s control poll — session
 * detail, queue, pending decisions — owns control-plane freshness until the
 * live tail reopens at the durable cursor on settle (Part II topology).
 */
export const PROMPT_POST_CONTROL_POLL_MS = 1_000;

export function useSessionRuntimeExtensions({
  eventSourceFactory,
  liveTailEnabled,
  promptDispatch,
  sessionId,
  workspaceId,
}: {
  eventSourceFactory?: SessionStreamEventSourceFactory;
  liveTailEnabled: boolean;
  promptDispatch: SessionPromptDispatchStore;
  sessionId: string;
  workspaceId: string;
}) {
  const aui = useAui();
  const hasLocalRuntimeTail = useSelector(
    promptDispatch,
    snapshot => snapshot.context.hasLocalRuntimeTail
  );
  const promptPending = useSelector(promptDispatch, snapshot => snapshot.context.pending);
  const liveTailSuppressed = useSelector(
    sessionStore,
    snapshot => (snapshot.context.liveTailSuppressions[sessionId] ?? 0) > 0
  );
  const streamResetGeneration = useSelector(
    promptDispatch,
    snapshot => snapshot.context.streamResetGeneration
  );
  const transcript = useMergedSessionRuntimeTranscript({
    eventSourceFactory,
    hasLocalRuntimeTail,
    // The prompt response is already the live source for its turn. Suspending
    // the parallel transcript SSE leaves an HTTP/1.1 connection available for
    // approval, clarification, cancel, and other control requests.
    liveTailEnabled: liveTailEnabled && !promptPending && !liveTailSuppressed,
    resetGeneration: streamResetGeneration,
    sessionId,
    workspaceId,
  });
  const controlPolling = liveTailEnabled && promptPending;
  const clarifications = useSessionClarifications(workspaceId, sessionId, {
    enabled: liveTailEnabled,
    refetchInterval: controlPolling ? PROMPT_POST_CONTROL_POLL_MS : false,
  });
  const inputs = useSessionInputs(workspaceId, sessionId, {
    enabled: liveTailEnabled,
    ...(controlPolling ? { refetchInterval: PROMPT_POST_CONTROL_POLL_MS } : {}),
  });
  // The session resource (state, activity, stop cause, queue summary) rides the
  // same bounded cadence; this observer exists only for the POST's duration.
  useQuery({
    ...sessionDetailOptions(workspaceId, sessionId),
    enabled: controlPolling,
    refetchInterval: PROMPT_POST_CONTROL_POLL_MS,
  });
  const { expiredInteractions, resolvedInteractions, rewindBlocked } = useSessionDecisionState({
    transcript,
    workspaceId,
    sessionId,
    liveTailEnabled,
    clarifications,
    inputs,
  });

  return {
    expiredInteractions,
    resolvedInteractions,
    resetRuntime: () => {
      promptDispatch.trigger.conversationReset();
      aui.thread.reset();
    },
    rewindBlocked,
    transcript,
  };
}

function useSessionDecisionState({
  transcript,
  workspaceId,
  sessionId,
  liveTailEnabled,
  clarifications,
  inputs,
}: {
  transcript: ReturnType<typeof useMergedSessionRuntimeTranscript>;
  workspaceId: string;
  sessionId: string;
  liveTailEnabled: boolean;
  clarifications: ReturnType<typeof useSessionClarifications>;
  inputs: ReturnType<typeof useSessionInputs>;
}) {
  const pendingPermissions = derivePendingPermissions(transcript.messages);
  // Only an undecided ask on screen can be a decision the daemon settled behind the
  // transcript's back; with none, the settled-interaction read never runs. The same
  // map decides which asks are genuinely open: those keep the read on its cadence
  // and keep blocking rewind; settled ones do neither.
  const undecidedRequestIds = new Set<string>([
    ...pendingPermissions.map(permission => permission.requestId),
    ...derivePendingClarifyRequestIds(transcript.messages),
  ]);
  const expiredInteractions = useSessionExpiredInteractions(workspaceId, sessionId, {
    enabled: liveTailEnabled && undecidedRequestIds.size > 0,
    undecidedRequestIds,
  });
  // A decided ask's receipt names who decided only from the daemon's resolved row; the
  // read runs while a receipt is on screen and re-reads once per new decision.
  const decidedRequestIds = deriveDecidedPermissionRequestIds(transcript.messages);
  const resolvedInteractions = useSessionResolvedInteractions(workspaceId, sessionId, {
    enabled: liveTailEnabled && decidedRequestIds.size > 0,
    decidedRequestIds,
  });
  const hasOpenPermission = pendingPermissions.some(
    permission => !expiredInteractions.has(permission.requestId)
  );
  const rewindBlocked =
    hasOpenPermission ||
    clarifications.isPending ||
    clarifications.isError ||
    (clarifications.data?.length ?? 0) > 0 ||
    inputs.isPending ||
    inputs.isError ||
    (inputs.data?.inputs.length ?? 0) > 0;

  return { expiredInteractions, resolvedInteractions, rewindBlocked };
}
