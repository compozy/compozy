import { useRef, useState, type ReactNode } from "react";
import { AssistantRuntimeProvider, DataRenderers, Tools, useAui } from "@assistant-ui/react";
import { useSelector } from "@xstate/store-react";

import { useMergedSessionRuntimeTranscript } from "../hooks/use-merged-session-runtime-transcript";
import { useSessionChatRuntime } from "../hooks/use-session-chat-runtime";
import { useSessionClarifications } from "../hooks/use-session-clarifications";
import { useSessionInputs } from "../hooks/use-session-inputs";
import { SessionPromptDispatchPendingProvider } from "@/components/assistant-ui/session-prompt-dispatch-context";
import {
  createSessionPromptDispatchStore,
  type SessionPromptDispatchStore,
} from "@/components/assistant-ui/session-prompt-dispatch-store";
import type { SessionStreamEventSourceFactory } from "../hooks/use-session-live-tail";
import { CompozyEventDataUI, CompozyPermissionDataUI } from "../lib/session-data-ui";
import { derivePendingPermissions } from "../lib/pending-permissions";
import { sessionToolkit } from "../lib/session-toolkit";
import { SessionRuntimeRenderProvider } from "../lib/session-runtime-render-context";
import { SessionTranscriptThreadProvider } from "../lib/session-transcript-thread-context";

function SessionRuntimeExtensions({
  sessionId,
  workspaceId,
  eventSourceFactory,
  liveTailEnabled,
  children,
}: {
  sessionId: string;
  workspaceId: string;
  eventSourceFactory?: SessionStreamEventSourceFactory;
  liveTailEnabled: boolean;
  children: ReactNode;
}) {
  const aui = useAui();
  const [streamResetGeneration, setStreamResetGeneration] = useState(0);
  const transcript = useMergedSessionRuntimeTranscript({
    eventSourceFactory,
    liveTailEnabled,
    resetGeneration: streamResetGeneration,
    sessionId,
    workspaceId,
  });
  const clarifications = useSessionClarifications(workspaceId, sessionId);
  const inputs = useSessionInputs(workspaceId, sessionId);
  const rewindBlocked =
    derivePendingPermissions(transcript.messages).length > 0 ||
    clarifications.isPending ||
    clarifications.isError ||
    (clarifications.data?.length ?? 0) > 0 ||
    inputs.isPending ||
    inputs.isError ||
    (inputs.data?.inputs.length ?? 0) > 0;
  const resetRuntime = () => {
    aui.thread.reset();
    setStreamResetGeneration(generation => generation + 1);
  };

  return (
    <SessionRuntimeRenderProvider
      resetRuntime={resetRuntime}
      rewindBlocked={rewindBlocked}
      sessionId={sessionId}
      workspaceId={workspaceId}
    >
      <SessionTranscriptThreadProvider
        liveMessages={transcript.liveMessages}
        messages={transcript.messages}
        status={transcript.status}
        error={transcript.error}
        hasOlder={transcript.hasOlder}
        isFetchingOlder={transcript.isFetchingOlder}
        loadOlder={transcript.loadOlder}
        retry={transcript.retry}
      >
        <CompozyPermissionDataUI />
        <CompozyEventDataUI />
        {children}
      </SessionTranscriptThreadProvider>
    </SessionRuntimeRenderProvider>
  );
}

function requireWorkspaceId(workspaceId: string): string {
  const trimmed = workspaceId.trim();
  if (!trimmed) {
    throw new Error("SessionChatRuntimeProvider requires a non-empty workspaceId");
  }
  return trimmed;
}

export interface SessionChatRuntimeProviderProps {
  sessionId: string;
  workspaceId: string;
  eventSourceFactory?: SessionStreamEventSourceFactory;
  liveTailEnabled?: boolean;
  children: ReactNode;
}

function SessionChatRuntimeBinding({
  sessionId,
  workspaceId,
  promptDispatch,
  eventSourceFactory,
  liveTailEnabled = true,
  children,
}: SessionChatRuntimeProviderProps & {
  promptDispatch: SessionPromptDispatchStore;
}) {
  const resolvedWorkspaceId = requireWorkspaceId(workspaceId);
  const runtime = useSessionChatRuntime({
    sessionId,
    workspaceId: resolvedWorkspaceId,
    promptDispatch,
  });
  const aui = useAui({
    tools: Tools({ toolkit: sessionToolkit }),
    dataRenderers: DataRenderers(),
  });

  return (
    <AssistantRuntimeProvider runtime={runtime} aui={aui}>
      <SessionPromptDispatchPendingProvider store={promptDispatch}>
        <SessionRuntimeExtensions
          sessionId={sessionId}
          workspaceId={resolvedWorkspaceId}
          eventSourceFactory={eventSourceFactory}
          liveTailEnabled={liveTailEnabled}
        >
          {children}
        </SessionRuntimeExtensions>
      </SessionPromptDispatchPendingProvider>
    </AssistantRuntimeProvider>
  );
}

export function SessionChatRuntimeProvider(props: SessionChatRuntimeProviderProps) {
  const promptDispatchRef = useRef<SessionPromptDispatchStore>(null);
  if (promptDispatchRef.current === null) {
    promptDispatchRef.current = createSessionPromptDispatchStore();
  }
  const promptDispatch = promptDispatchRef.current;
  const runtimeGeneration = useSelector(promptDispatch, snapshot => snapshot.context.generation);

  return (
    <SessionChatRuntimeBinding {...props} key={runtimeGeneration} promptDispatch={promptDispatch} />
  );
}
