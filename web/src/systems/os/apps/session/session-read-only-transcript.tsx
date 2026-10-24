import { useTopbarSlot } from "@compozy/ui";
import {
  getSessionDisplayTitle,
  SessionChatRuntimeProvider,
  type SessionPayload,
} from "@/systems/session";

import { SessionThread } from "./session-thread-lazy";

/** Shared retained-history surface for Global and another profile's sessions. */
export function SessionReadOnlyTranscript({
  session,
  agentName,
  liveTailEnabled,
}: {
  session: SessionPayload;
  agentName: string;
  liveTailEnabled: boolean;
}) {
  const workspaceId = session.workspace_id?.trim() ?? "";
  const liveDataEnabled = liveTailEnabled && workspaceId !== "";
  useTopbarSlot({ crumb: getSessionDisplayTitle(session) });
  return (
    <SessionChatRuntimeProvider
      sessionId={session.id}
      workspaceId={workspaceId}
      liveTailEnabled={liveDataEnabled}
      readOnly
    >
      <SessionThread
        readOnly
        canPrompt={false}
        liveDataEnabled={liveDataEnabled}
        sessionId={session.id}
        workspaceId={workspaceId}
        agentName={session.agent_name || agentName}
        acpSessionId={session.runtime.acp_session_id}
        sessionState={session.state}
        failure={session.failure}
      />
    </SessionChatRuntimeProvider>
  );
}
