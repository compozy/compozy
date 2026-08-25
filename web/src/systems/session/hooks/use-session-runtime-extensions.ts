import { useAui } from "@assistant-ui/react";
import { useSelector } from "@xstate/store-react";

import type { SessionPromptDispatchStore } from "@/components/assistant-ui/session-prompt-dispatch-store";
import { derivePendingPermissions } from "../lib/pending-permissions";
import { sessionStore } from "../stores/session-store";
import { useMergedSessionRuntimeTranscript } from "./use-merged-session-runtime-transcript";
import { useSessionClarifications } from "./use-session-clarifications";
import { useSessionInputs } from "./use-session-inputs";
import type { SessionStreamEventSourceFactory } from "./use-session-live-tail";

const ACTIVE_PROMPT_CLARIFICATION_REFETCH_MS = 1_000;

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
  const clarifications = useSessionClarifications(workspaceId, sessionId, {
    enabled: liveTailEnabled,
    refetchInterval: promptPending ? ACTIVE_PROMPT_CLARIFICATION_REFETCH_MS : false,
  });
  const inputs = useSessionInputs(workspaceId, sessionId, { enabled: liveTailEnabled });
  const rewindBlocked =
    derivePendingPermissions(transcript.messages).length > 0 ||
    clarifications.isPending ||
    clarifications.isError ||
    (clarifications.data?.length ?? 0) > 0 ||
    inputs.isPending ||
    inputs.isError ||
    (inputs.data?.inputs.length ?? 0) > 0;

  return {
    resetRuntime: () => {
      promptDispatch.trigger.conversationReset();
      aui.thread.reset();
    },
    rewindBlocked,
    transcript,
  };
}
