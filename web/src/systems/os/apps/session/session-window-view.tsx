import { lazy } from "react";
import { AlertCircle } from "lucide-react";

import { Spinner } from "@compozy/ui";

import { loadSessionWindowContent } from "./session-window-module-loader";
import {
  canPromptSession,
  SessionChatRuntimeProvider,
  type SessionPayload,
  SessionPromptRuntimeProvider,
} from "@/systems/session";

const SessionWindowContent = lazy(() =>
  loadSessionWindowContent().then(module => ({ default: module.SessionWindowContent }))
);

export function SessionWindowView({
  windowId,
  name,
  id,
  workspaceId,
  session,
  liveTailEnabled,
  isLoading,
  error,
  onDeleteSuccess,
}: {
  windowId: string;
  name: string;
  id: string;
  workspaceId: string;
  session: SessionPayload | undefined;
  liveTailEnabled: boolean;
  isLoading: boolean;
  error: Error | null;
  onDeleteSuccess: () => void;
}) {
  const sessionWorkspaceId = session?.workspace_id?.trim();

  if (isLoading) {
    return (
      <div
        className="flex min-h-full items-center justify-center"
        data-testid="session-route-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }
  if (!session || sessionWorkspaceId !== workspaceId) {
    return <SessionWindowNotice message={error?.message ?? "Session not found"} />;
  }

  const resolvedAgentName = session.agent_name || name;
  return (
    <SessionPromptRuntimeProvider key={id} canPrompt={canPromptSession(session)} session={session}>
      <SessionChatRuntimeProvider
        sessionId={id}
        workspaceId={sessionWorkspaceId}
        liveTailEnabled={liveTailEnabled}
      >
        <SessionWindowContent
          liveDataEnabled={liveTailEnabled}
          windowId={windowId}
          agentName={resolvedAgentName}
          sessionId={id}
          session={session}
          workspaceId={sessionWorkspaceId}
          onDeleteSuccess={onDeleteSuccess}
        />
      </SessionChatRuntimeProvider>
    </SessionPromptRuntimeProvider>
  );
}

export function SessionWindowNotice({ message }: { message: string }) {
  return (
    <div className="flex min-h-full items-center justify-center">
      <div className="flex flex-col items-center gap-2 text-center">
        <AlertCircle className="size-6 text-danger" />
        <p className="text-small-body text-subtle">{message}</p>
      </div>
    </div>
  );
}
