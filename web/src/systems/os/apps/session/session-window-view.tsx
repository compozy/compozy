import { lazy, Suspense } from "react";
import { AlertCircle } from "lucide-react";

import { Empty } from "@compozy/ui";

import { loadSessionWindowContent } from "./session-window-module-loader";
import { SessionReadOnlyTranscript } from "./session-read-only-transcript";
import { SessionWindowSkeleton } from "./session-window-skeleton";
import {
  canPromptSession,
  SessionChatRuntimeProvider,
  type SessionPayload,
  SessionPromptRuntimeProvider,
} from "@/systems/session";
import { USER_SCOPE_SELECTOR } from "@/systems/workspace";

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
}: {
  windowId: string;
  name: string;
  id: string;
  workspaceId: string;
  session: SessionPayload | undefined;
  liveTailEnabled: boolean;
  isLoading: boolean;
  error: Error | null;
}) {
  const sessionWorkspaceId = session ? (session.workspace_id?.trim() ?? "") : undefined;

  if (isLoading) return <SessionWindowSkeleton />;
  if (!session || sessionWorkspaceId !== workspaceId) {
    return <SessionWindowNotice title="Couldn't find this session" detail={error?.message} />;
  }

  const resolvedAgentName = session.agent_name || name;
  // A User-scope session has no workspace but is live work; only legacy sessions without
  // any scope fall back to the read-only history.
  const runtimeWorkspaceId = session.scope === "user" ? USER_SCOPE_SELECTOR : sessionWorkspaceId;
  if (runtimeWorkspaceId === "") {
    return (
      <SessionReadOnlyTranscript
        session={session}
        agentName={resolvedAgentName}
        liveTailEnabled={false}
      />
    );
  }
  return (
    <Suspense fallback={<SessionWindowSkeleton />}>
      <SessionPromptRuntimeProvider
        key={id}
        canPrompt={canPromptSession(session)}
        session={session}
      >
        <SessionChatRuntimeProvider
          sessionId={id}
          workspaceId={runtimeWorkspaceId}
          liveTailEnabled={liveTailEnabled}
        >
          <SessionWindowContent
            liveDataEnabled={liveTailEnabled}
            windowId={windowId}
            agentName={resolvedAgentName}
            sessionId={id}
            session={session}
            workspaceId={runtimeWorkspaceId}
          />
        </SessionChatRuntimeProvider>
      </SessionPromptRuntimeProvider>
    </Suspense>
  );
}

/**
 * A session the window cannot show. The headline is always plain copy; a raw
 * error message, when there is one, stays behind Empty's "Details" fold.
 */
export function SessionWindowNotice({ title, detail }: { title: string; detail?: string | null }) {
  return (
    <Empty
      className="min-h-full"
      icon={AlertCircle}
      role="alert"
      size="compact"
      title={title}
      cause={detail || undefined}
    />
  );
}
