import type { ReactNode } from "react";
import { AssistantRuntimeProvider, DataRenderers, Tools, useAui } from "@assistant-ui/react";

import { useMergedSessionRuntimeTranscript } from "../hooks/use-merged-session-runtime-transcript";
import { useSessionChatRuntime } from "../hooks/use-session-chat-runtime";
import type { SessionStreamEventSourceFactory } from "../hooks/use-session-live-tail";
import { AghEventDataUI, AghPermissionDataUI } from "../lib/session-data-ui";
import { sessionToolkit } from "../lib/session-toolkit";
import { SessionRuntimeRenderProvider } from "../lib/session-runtime-render-context";
import { SessionTranscriptThreadProvider } from "../lib/session-transcript-thread-context";

function SessionRuntimeExtensions({
  sessionId,
  workspaceId,
  eventSourceFactory,
  children,
}: {
  sessionId: string;
  workspaceId: string;
  eventSourceFactory?: SessionStreamEventSourceFactory;
  children: ReactNode;
}) {
  const transcript = useMergedSessionRuntimeTranscript({
    eventSourceFactory,
    sessionId,
    workspaceId,
  });

  return (
    <SessionRuntimeRenderProvider sessionId={sessionId} workspaceId={workspaceId}>
      <SessionTranscriptThreadProvider
        messages={transcript.messages}
        status={transcript.status}
        error={transcript.error}
        hasOlder={transcript.hasOlder}
        isFetchingOlder={transcript.isFetchingOlder}
        loadOlder={transcript.loadOlder}
        retry={transcript.retry}
      >
        <AghPermissionDataUI />
        <AghEventDataUI />
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
  children: ReactNode;
}

export function SessionChatRuntimeProvider({
  sessionId,
  workspaceId,
  eventSourceFactory,
  children,
}: SessionChatRuntimeProviderProps) {
  const resolvedWorkspaceId = requireWorkspaceId(workspaceId);
  const runtime = useSessionChatRuntime({ sessionId, workspaceId: resolvedWorkspaceId });
  const aui = useAui({
    tools: Tools({ toolkit: sessionToolkit }),
    dataRenderers: DataRenderers(),
  });

  return (
    <AssistantRuntimeProvider runtime={runtime} aui={aui}>
      <SessionRuntimeExtensions
        sessionId={sessionId}
        workspaceId={resolvedWorkspaceId}
        eventSourceFactory={eventSourceFactory}
      >
        {children}
      </SessionRuntimeExtensions>
    </AssistantRuntimeProvider>
  );
}
