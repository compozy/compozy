import { lazy, Suspense, ViewTransition } from "react";
import { useAgentInstances } from "@/systems/agent";
import { AlertCircle, History } from "@compozy/ui/icons";

import { Empty, VIEW_TRANSITION_CLASS } from "@compozy/ui";

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
  const instances = useAgentInstances();
  const instance = instances.find(row => row.id === session?.instance_id);
  const currentSessionId = instance?.main_session_id;
  const superseded = Boolean(currentSessionId && currentSessionId !== session?.id);
  const sessionWorkspaceId = session ? (session.workspace_id?.trim() ?? "") : undefined;

  if (isLoading) return <SessionWindowSkeleton />;
  if (!session || sessionWorkspaceId !== workspaceId) {
    return <SessionWindowNotice title="Couldn't find this session" detail={error?.message} />;
  }

  const resolvedAgentName = session.agent_name || name;
  if (superseded && currentSessionId) {
    return (
      <>
        <SessionReadOnlyTranscript
          session={session}
          agentName={resolvedAgentName}
          liveTailEnabled={false}
        />
        <div className="mx-auto flex w-full max-w-3xl items-center gap-3 px-6 pt-2 pb-5">
          <div className="flex min-w-0 flex-1 items-center gap-3 rounded-lg bg-surface-2 px-3 py-2.5 shadow-hairline-inset">
            <History aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 text-small-body text-muted-foreground">
              This conversation moved on. You can read it here.
            </span>
            <a
              className="text-small-body text-fg underline"
              href={`/session/${encodeURIComponent(currentSessionId)}`}
            >
              Open the current one
            </a>
          </div>
        </div>
      </>
    );
  }

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
  // The one animated seam of this window: switching to another session in place. The
  // boundary is keyed by the session, so the switch cross-fades the departing window body
  // with the arriving one and nothing else does (`default="none"`: a Suspense reveal, a
  // live update or a tab swap elsewhere never snapshots it). The caller defers the session
  // identity (`useSessionWindowRuntime`), which is what makes the switch a transition.
  return (
    <ViewTransition
      key={id}
      default="none"
      enter={VIEW_TRANSITION_CLASS.fade}
      exit={VIEW_TRANSITION_CLASS.fade}
    >
      <Suspense fallback={<SessionWindowSkeleton />}>
        <SessionPromptRuntimeProvider canPrompt={canPromptSession(session)} session={session}>
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
    </ViewTransition>
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
