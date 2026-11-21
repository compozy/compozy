import { lazy, Suspense, ViewTransition } from "react";
import { useAgentInstances } from "@/systems/agent";
import { AlertCircle, History } from "@compozy/ui/icons";

import { Button, Empty, Spinner, VIEW_TRANSITION_CLASS } from "@compozy/ui";

import { loadSessionWindowContent } from "./session-window-module-loader";
import { SessionReadOnlyTranscript } from "./session-read-only-transcript";
import { SessionWindowRail } from "./session-window-rail";
import { SessionWindowSkeleton } from "./session-window-skeleton";
import {
  useMarkSeenOnLeave,
  canPromptSession,
  SessionChatRuntimeProvider,
  type SessionPayload,
  SessionPromptRuntimeProvider,
} from "@/systems/session";
import { isSessionUnavailable } from "@/systems/session/adapters/session-api-errors";
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
  focused = false,
  liveTailEnabled,
  isLoading,
  error,
  onRetry,
  retrying = false,
}: {
  windowId: string;
  name: string;
  id: string;
  workspaceId: string;
  session: SessionPayload | undefined;
  focused?: boolean;
  liveTailEnabled: boolean;
  isLoading: boolean;
  error: Error | null;
  /** Reads the session again; offered when the read failed for a reason other than not-found. */
  onRetry?: () => void;
  retrying?: boolean;
}) {
  const instances = useAgentInstances();
  const instance = instances.find(row => row.id === session?.instance_id);
  const currentSessionId = instance?.main_session_id;
  const superseded = Boolean(currentSessionId && currentSessionId !== session?.id);
  useMarkSeenOnLeave(instance, Boolean(session) && currentSessionId === session?.id);
  const sessionWorkspaceId = session?.workspace_id?.trim() ?? "";

  // The rail is the window's: it frames the next session's loading state and its loaded pane
  // alike, so a row pick never remounts the list or resets its scroll (#1923).
  if (isLoading) {
    return (
      <SessionWindowRail windowId={windowId} sessionId={id} session={undefined}>
        <SessionWindowSkeleton />
      </SessionWindowRail>
    );
  }
  const placeholder = sessionWindowPlaceholder({
    shown: session !== undefined && sessionWorkspaceId === workspaceId,
    error,
    onRetry,
    retrying,
  });
  if (placeholder || !session) return placeholder;

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
  // The keyed boundary replaces the departing session's providers and composer together; the
  // window's rail stays outside it (#1923). Route identity is urgent: the next session or its
  // loading state must replace the old conversation without waiting for an animation. The
  // boundary can still animate an eligible transition; live updates and unrelated tab swaps
  // never snapshot it.
  return (
    <SessionWindowRail windowId={windowId} sessionId={id} session={session}>
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
                focused={focused}
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
    </SessionWindowRail>
  );
}

/**
 * What the window shows instead of a loaded session: a retryable notice when the
 * read failed for any reason but not-found, and "Couldn't find this session" only
 * for a session that is not there (or not in this workspace). `null` once the session can be shown.
 */
function sessionWindowPlaceholder({
  shown,
  error,
  onRetry,
  retrying,
}: {
  shown: boolean;
  error: Error | null;
  onRetry?: () => void;
  retrying: boolean;
}): React.ReactNode {
  if (shown) return null;
  if (error && !isSessionUnavailable(error)) {
    return (
      <SessionWindowNotice
        title="Couldn't load this session"
        detail={error.message}
        action={onRetry ? <SessionWindowRetry onRetry={onRetry} retrying={retrying} /> : undefined}
      />
    );
  }
  return <SessionWindowNotice title="Couldn't find this session" detail={error?.message} />;
}

function SessionWindowRetry({ onRetry, retrying }: { onRetry: () => void; retrying: boolean }) {
  return (
    <Button
      data-testid="session-window-retry"
      disabled={retrying}
      onClick={onRetry}
      size="sm"
      type="button"
      variant="secondary"
    >
      {retrying ? <Spinner aria-hidden="true" className="size-3" /> : null}
      Try again
    </Button>
  );
}

/**
 * A session the window cannot show. The headline is always plain copy; a raw
 * error message, when there is one, stays behind Empty's "Details" fold.
 */
export function SessionWindowNotice({
  title,
  detail,
  action,
}: {
  title: string;
  detail?: string | null;
  action?: React.ReactNode;
}) {
  return (
    <Empty
      className="min-h-full"
      icon={AlertCircle}
      role="alert"
      size="compact"
      title={title}
      cause={detail || undefined}
      action={action}
    />
  );
}
