import type { UIMessage } from "ai";
import { useEffect, useState, type ReactNode } from "react";
import {
  AssistantRuntimeProvider,
  DataRenderers,
  Tools,
  useAui,
  useAuiState,
} from "@assistant-ui/react";
import { useStore } from "@xstate/store-react";
import { toast } from "@compozy/ui";

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
  sessionPromptRecovery,
  type RestoredSessionPromptDraft,
  type SessionPromptRecovery,
} from "@/systems/session/lib/session-prompt-recovery";
import { stageChosenSessionTerminalQuote } from "@/systems/session/lib/session-terminal-quote";
import { SessionTranscriptThreadProvider } from "@/systems/session/lib/session-transcript-thread-context";

/** Returns false, touching nothing, when the composer already holds another draft. */
async function restoreRejectedPromptDraft(
  aui: ReturnType<typeof useAui>,
  draft: RestoredSessionPromptDraft,
  sessionId: string
): Promise<boolean> {
  const composer = aui.composer.getState();
  if (composer.text.trim().length > 0 || composer.attachments.length > 0) return false;
  if (draft.quote) {
    stageChosenSessionTerminalQuote(sessionId, draft.quote);
  }
  aui.composer.setText(draft.text);
  await draft.files.reduce(
    (pending, file) => pending.then(() => aui.composer.addAttachment(file)),
    Promise.resolve()
  );
  return true;
}

function SessionPromptRecoveryBridge({
  recovery,
  sessionId,
  workspaceId,
}: {
  recovery: SessionPromptRecovery;
  sessionId: string;
  workspaceId: string;
}) {
  const aui = useAui();
  // A refused draft returns whole — its own text and files — into an empty composer, in
  // arrival order. While the operator holds another draft there, it waits instead of losing
  // its text or folding its files into someone else's message.
  const [restore, setRestore] = useState<{
    queue: readonly RestoredSessionPromptDraft[];
    restoring: RestoredSessionPromptDraft | null;
  }>({ queue: [], restoring: null });
  const composerEmpty = useAuiState(
    state => state.composer.text.trim().length === 0 && state.composer.attachments.length === 0
  );
  const draft = restore.restoring ?? (composerEmpty ? (restore.queue[0] ?? null) : null);

  useEffect(
    () =>
      recovery.subscribe({ workspaceId, sessionId }, restored =>
        setRestore(current =>
          current.queue.includes(restored)
            ? current
            : { ...current, queue: [...current.queue, restored] }
        )
      ),
    [recovery, sessionId, workspaceId]
  );

  useEffect(() => {
    if (draft === null) return;
    let active = true;
    // assistant-ui finalizes the submitted composer after its onError callback.
    // Restore on the next task so that finalization cannot erase the recovered draft.
    const timeout = window.setTimeout(() => {
      // The restore fills the composer; pin the draft so that change cannot cancel it.
      setRestore(current => ({ ...current, restoring: draft }));
      let restored = true;
      void restoreRejectedPromptDraft(aui, draft, sessionId)
        .then(result => {
          restored = result;
        })
        .catch(() => {
          toast.error("Couldn't restore the rejected attachment.");
        })
        .finally(() => {
          // A composer torn down mid-restore leaves the draft with recovery for the next one.
          if (!active) return;
          if (restored) recovery.settle({ workspaceId, sessionId }, draft);
          setRestore(current => ({
            queue: restored
              ? current.queue.filter(candidate => candidate !== draft)
              : current.queue,
            restoring: null,
          }));
        });
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timeout);
    };
  }, [aui, draft, recovery, sessionId, workspaceId]);

  return null;
}

function SessionRuntimeExtensions({
  sendDetachedPrompt,
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
  sendDetachedPrompt: (message: UIMessage) => Promise<void>;
  eventSourceFactory?: SessionStreamEventSourceFactory;
  liveTailEnabled: boolean;
  children: ReactNode;
}) {
  const {
    expiredInteractions,
    resolvedInteractions,
    resetRuntime,
    rewindBlocked,
    sessionRunning,
    transcript,
  } = useSessionRuntimeExtensions({
    eventSourceFactory,
    liveTailEnabled,
    promptDispatch,
    sessionId,
    workspaceId,
  });

  return (
    <SessionRuntimeRenderProvider
      durableMessageIds={transcript.durableMessageIds}
      expiredInteractions={expiredInteractions}
      resolvedInteractions={resolvedInteractions}
      resetRuntime={resetRuntime}
      sendDetachedPrompt={sendDetachedPrompt}
      rewindBlocked={rewindBlocked}
      sessionRunning={sessionRunning}
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
        transport={{ ...transcript.transport, retry: transcript.retry }}
      >
        <CompozyPermissionDataUI />
        <CompozyEventDataUI />
        {children}
      </SessionTranscriptThreadProvider>
    </SessionRuntimeRenderProvider>
  );
}

function requireWorkspaceId(workspaceId: string, readOnly: boolean): string {
  const trimmed = workspaceId.trim();
  if (!trimmed && !readOnly) {
    throw new Error("SessionChatRuntimeProvider requires a non-empty workspaceId");
  }
  return trimmed;
}

export interface SessionChatRuntimeProviderProps {
  sessionId: string;
  workspaceId: string;
  eventSourceFactory?: SessionStreamEventSourceFactory;
  liveTailEnabled?: boolean;
  /** Retained history can render without a project; prompting remains unavailable. */
  readOnly?: boolean;
  children: ReactNode;
}

function SessionChatRuntimeBinding({
  sessionId,
  workspaceId,
  promptDispatch,
  promptRecovery,
  eventSourceFactory,
  liveTailEnabled = true,
  readOnly = false,
  children,
}: SessionChatRuntimeProviderProps & {
  promptDispatch: SessionPromptDispatchStore;
  promptRecovery: SessionPromptRecovery;
}) {
  const resolvedWorkspaceId = requireWorkspaceId(workspaceId, readOnly);
  const { runtime, sendDetachedPrompt } = useSessionChatRuntime({
    sessionId,
    workspaceId: resolvedWorkspaceId,
    promptDispatch,
    promptRecovery,
    readOnly,
  });
  const aui = useAui({
    tools: Tools({ toolkit: sessionToolkit }),
    dataRenderers: DataRenderers(),
  });

  return (
    <AssistantRuntimeProvider runtime={runtime} aui={aui}>
      <SessionPromptDispatchPendingProvider store={promptDispatch}>
        <SessionPromptRecoveryBridge
          recovery={promptRecovery}
          sessionId={sessionId}
          workspaceId={resolvedWorkspaceId}
        />
        <SessionRuntimeExtensions
          sendDetachedPrompt={sendDetachedPrompt}
          sessionId={sessionId}
          workspaceId={resolvedWorkspaceId}
          promptDispatch={promptDispatch}
          eventSourceFactory={eventSourceFactory}
          liveTailEnabled={liveTailEnabled && resolvedWorkspaceId !== ""}
        >
          {children}
        </SessionRuntimeExtensions>
      </SessionPromptDispatchPendingProvider>
    </AssistantRuntimeProvider>
  );
}

/**
 * The runtime identity is the session identity. Switching workspace or session
 * rebuilds the runtime; cancelling, stopping, or steering a turn never does
 * (US-019.EC-1): the dispatch store's `pendingCanceled` aborts the tracked
 * prompt fetch, which the AI SDK reads as a clean cancel, while the composer
 * draft, attachments, reading position, and the interrupted turn stay put.
 */
export function SessionChatRuntimeProvider(props: SessionChatRuntimeProviderProps) {
  const promptDispatch = useStore(sessionPromptDispatchLogic);

  return (
    <SessionChatRuntimeBinding
      {...props}
      key={`${props.workspaceId}:${props.sessionId}`}
      promptDispatch={promptDispatch}
      promptRecovery={sessionPromptRecovery}
    />
  );
}
