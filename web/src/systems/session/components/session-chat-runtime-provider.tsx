import { useEffect, useState, type ReactNode } from "react";
import { AssistantRuntimeProvider, DataRenderers, Tools, useAui } from "@assistant-ui/react";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { SessionPromptDispatchPendingProvider } from "@/components/assistant-ui/session-prompt-dispatch-context";
import {
  sessionPromptDispatchLogic,
  type SessionPromptDispatchStore,
} from "@/components/assistant-ui/session-prompt-dispatch-store";
import { useSessionChatRuntime } from "@/systems/session/hooks/use-session-chat-runtime";
import type { SessionStreamEventSourceFactory } from "@/systems/session/hooks/use-session-live-tail";
import { useSessionRuntimeExtensions } from "@/systems/session/hooks/use-session-runtime-extensions";
import { CompozyEventDataUI, CompozyPermissionDataUI } from "@/systems/session/lib/session-data-ui";
import { sessionToolkit } from "@/systems/session/lib/session-toolkit";
import { SessionRuntimeRenderProvider } from "@/systems/session/lib/session-runtime-render-context";
import {
  SessionPromptRecovery,
  type RestoredSessionPromptDraft,
} from "@/systems/session/lib/session-prompt-recovery";
import { SessionTranscriptThreadProvider } from "@/systems/session/lib/session-transcript-thread-context";

async function restoreRejectedPromptDraft(
  aui: ReturnType<typeof useAui>,
  draft: RestoredSessionPromptDraft
): Promise<void> {
  if (aui.composer.getState().text.length === 0) {
    aui.composer.setText(draft.text);
  }
  await Promise.all(draft.files.map(file => aui.composer.addAttachment(file)));
}

function SessionPromptRecoveryBridge({ recovery }: { recovery: SessionPromptRecovery }) {
  const aui = useAui();
  const [draft, setDraft] = useState<RestoredSessionPromptDraft | null>(null);

  useEffect(() => recovery.subscribe(setDraft), [recovery]);

  useEffect(() => {
    if (draft === null) return;
    let active = true;
    // assistant-ui finalizes the submitted composer after its onError callback.
    // Restore on the next task so that finalization cannot erase the recovered draft.
    const timeout = window.setTimeout(() => {
      void restoreRejectedPromptDraft(aui, draft)
        .catch(() => {
          toast.error("Couldn't restore the rejected attachment.");
        })
        .finally(() => {
          if (active) {
            setDraft(current => (current === draft ? null : current));
          }
        });
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timeout);
    };
  }, [aui, draft]);

  return null;
}

function SessionRuntimeExtensions({
  sessionId,
  workspaceId,
  promptDispatch,
  eventSourceFactory,
  liveTailEnabled,
  children,
}: {
  sessionId: string;
  workspaceId: string;
  promptDispatch: SessionPromptDispatchStore;
  eventSourceFactory?: SessionStreamEventSourceFactory;
  liveTailEnabled: boolean;
  children: ReactNode;
}) {
  const { resetRuntime, rewindBlocked, transcript } = useSessionRuntimeExtensions({
    eventSourceFactory,
    liveTailEnabled,
    promptDispatch,
    sessionId,
    workspaceId,
  });

  return (
    <SessionRuntimeRenderProvider
      durableMessageIds={transcript.durableMessageIds}
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
  promptRecovery,
  eventSourceFactory,
  liveTailEnabled = true,
  children,
}: SessionChatRuntimeProviderProps & {
  promptDispatch: SessionPromptDispatchStore;
  promptRecovery: SessionPromptRecovery;
}) {
  const resolvedWorkspaceId = requireWorkspaceId(workspaceId);
  const runtime = useSessionChatRuntime({
    sessionId,
    workspaceId: resolvedWorkspaceId,
    promptDispatch,
    promptRecovery,
  });
  const aui = useAui({
    tools: Tools({ toolkit: sessionToolkit }),
    dataRenderers: DataRenderers(),
  });

  return (
    <AssistantRuntimeProvider runtime={runtime} aui={aui}>
      <SessionPromptDispatchPendingProvider store={promptDispatch}>
        <SessionPromptRecoveryBridge recovery={promptRecovery} />
        <SessionRuntimeExtensions
          sessionId={sessionId}
          workspaceId={resolvedWorkspaceId}
          promptDispatch={promptDispatch}
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
  const promptDispatch = useStore(sessionPromptDispatchLogic);
  const [promptRecovery] = useState(() => new SessionPromptRecovery());
  const runtimeGeneration = useSelector(promptDispatch, snapshot => snapshot.context.generation);

  return (
    <SessionChatRuntimeBinding
      {...props}
      key={`${props.workspaceId}:${props.sessionId}:${runtimeGeneration}`}
      promptDispatch={promptDispatch}
      promptRecovery={promptRecovery}
    />
  );
}
