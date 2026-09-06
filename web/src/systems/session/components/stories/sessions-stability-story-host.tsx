// Story host for the sessions-stability visual states (task_10 matrix): the
// production session thread inside the artboard's session-window body, over
// the real runtime provider and MSW transcript/search/outline routes. Head
// chrome is placement context only (title + the production transport chip);
// every rendered piece below it is the shipped component.

import { type ReactNode, useEffect } from "react";

import { Eyebrow } from "@compozy/ui";

import type { SessionComposerProps } from "@/components/assistant-ui/session-composer";
import { SessionThread } from "@/components/assistant-ui/session-thread";
import { ThreadContentRail } from "@/components/assistant-ui/session-thread-content-rail";
import { SESSION_THREAD_CONTENT_INSET_DEFAULT } from "@/components/assistant-ui/session-thread-content-rail-constants";
import type { SessionThreadStatusSession } from "@/components/assistant-ui/session-thread-status-row";
import { cn } from "@/lib/utils";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import { SessionQuietWarningNotice } from "@/systems/session/components/session-quiet-warning-notice";
import { SessionTransportChip } from "@/systems/session/components/session-transport-chip";
import type { SessionQuietWarning } from "@/systems/session/lib/session-quiet-warning";
import {
  SessionTransportContext,
  type SessionTransportState,
} from "@/systems/session/lib/session-transcript-thread-context-value";
import { sessionStreamingPreferenceStore } from "@/systems/session/stores/session-streaming-preference-store";

import { STABILITY_SESSION, STABILITY_WORKSPACE_ID } from "./sessions-stability-story-fixtures";

export interface StabilityThreadHostProps {
  /** The session resource the status row reads; defaults to the settled fixture. */
  statusSession?: SessionThreadStatusSession | null;
  isSessionRunning?: boolean;
  quietWarning?: SessionQuietWarning | null;
  liveDataEnabled?: boolean;
  stopCompletionNote?: boolean;
  /** Injected transport snapshot (the production context the chip, notices, and composer guard read). */
  transport?: SessionTransportState;
  /** `false` renders the head's chip as a background (paused) window. */
  windowLive?: boolean;
  /** Window body width; the artboards show sessions at 860px, the trail needs ≥864px. */
  width?: number;
  height?: number;
  composerProps?: Pick<
    SessionComposerProps,
    "stopPhase" | "onSteerPrompt" | "onQueuePrompt" | "busyInputDefaultMode"
  >;
}

function TransportScope({
  transport,
  children,
}: {
  transport: SessionTransportState | undefined;
  children: ReactNode;
}) {
  if (!transport) return children;
  return (
    <SessionTransportContext.Provider value={transport}>
      {children}
    </SessionTransportContext.Provider>
  );
}

/**
 * The session window body: a head line with the session name, the agent, and
 * the transport chip slot at its trailing edge; the notice rail; the thread.
 */
export function StabilityThreadHost({
  statusSession = STABILITY_SESSION,
  isSessionRunning = false,
  quietWarning = null,
  liveDataEnabled = true,
  stopCompletionNote = false,
  transport,
  windowLive = true,
  width = 860,
  height = 560,
  composerProps,
}: StabilityThreadHostProps) {
  return (
    <SessionChatRuntimeProvider
      sessionId={STABILITY_SESSION.id}
      workspaceId={STABILITY_WORKSPACE_ID}
      liveTailEnabled={false}
    >
      <TransportScope transport={transport}>
        <div
          className={cn(
            "flex max-w-full flex-col overflow-hidden rounded-window border border-line-focus bg-canvas shadow-window"
          )}
          data-testid="stability-story-window"
          style={{ height, width }}
        >
          <div className="flex h-9 shrink-0 items-center gap-3 border-b border-line px-4">
            <span className="text-small-body font-medium text-fg">{STABILITY_SESSION.name}</span>
            <Eyebrow>{STABILITY_SESSION.agent_name}</Eyebrow>
            <div className="ml-auto flex items-center">
              <SessionTransportChip windowLive={windowLive} />
            </div>
          </div>
          {quietWarning ? (
            <ThreadContentRail inset={SESSION_THREAD_CONTENT_INSET_DEFAULT}>
              <SessionQuietWarningNotice
                isStopping={false}
                onStop={() => undefined}
                warning={quietWarning}
              />
            </ThreadContentRail>
          ) : null}
          <SessionThread
            sessionId={STABILITY_SESSION.id}
            workspaceId={STABILITY_WORKSPACE_ID}
            agentName={STABILITY_SESSION.agent_name}
            sessionState={statusSession?.state ?? STABILITY_SESSION.state}
            statusSession={statusSession}
            quietWarning={quietWarning}
            liveDataEnabled={liveDataEnabled}
            stopCompletionNote={stopCompletionNote}
            isSessionRunning={isSessionRunning}
            canPrompt
            onCancelPrompt={() => undefined}
            onQueuePrompt={() => undefined}
            onSteerPrompt={() => undefined}
            onInterruptPrompt={() => undefined}
            {...composerProps}
          />
        </div>
      </TransportScope>
    </SessionChatRuntimeProvider>
  );
}

/** Turns the Smooth streaming preference off for the story's lifetime (S10, timeline VC-09 "off"). */
export function SmoothStreamingOff({ children }: { children: ReactNode }) {
  useEffect(() => {
    const previous = sessionStreamingPreferenceStore.getSnapshot().context.smoothStreaming;
    sessionStreamingPreferenceStore.trigger.smoothStreamingSet({ enabled: false });
    return () => {
      sessionStreamingPreferenceStore.trigger.smoothStreamingSet({ enabled: previous });
    };
  }, []);
  return children;
}
