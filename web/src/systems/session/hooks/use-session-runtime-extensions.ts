import { useAui } from "@assistant-ui/react";
import { useSelector } from "@xstate/store-react";

import type { SessionPromptDispatchStore } from "@/components/assistant-ui/session-prompt-dispatch-store";
import { derivePendingPermissions } from "../lib/pending-permissions";
import { useMergedSessionRuntimeTranscript } from "./use-merged-session-runtime-transcript";
import { useSessionClarifications } from "./use-session-clarifications";
import { useSessionInputs } from "./use-session-inputs";
import type { SessionStreamEventSourceFactory } from "./use-session-live-tail";

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
  const streamResetGeneration = useSelector(
    promptDispatch,
    snapshot => snapshot.context.streamResetGeneration
  );
  const transcript = useMergedSessionRuntimeTranscript({
    eventSourceFactory,
    hasLocalRuntimeTail,
    liveTailEnabled,
    resetGeneration: streamResetGeneration,
    sessionId,
    workspaceId,
  });
  const clarifications = useSessionClarifications(workspaceId, sessionId, {
    enabled: liveTailEnabled,
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
