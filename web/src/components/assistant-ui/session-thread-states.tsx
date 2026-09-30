import { Link } from "@tanstack/react-router";
import { AlertCircle, WifiOff } from "lucide-react";

import { Button, Empty, Skeleton, Spinner } from "@compozy/ui";

import { cn } from "@/lib/utils";
import type {
  SessionFailurePayload,
  SessionState,
  SessionTranscriptThreadStatus,
} from "@/systems/session";

import { formatMessageError } from "./session-thread-error";

const STATE_PANE_FRAME = "flex min-h-full w-full min-w-0 flex-1 items-center justify-center py-12";
const RUNTIME_RECOVERY_FAILURE_KINDS = new Set<SessionFailurePayload["kind"]>([
  "provider_auth_failure",
  "handshake_failure",
  "transport_failure",
  "timeout",
]);

/**
 * Placeholder assistant row — mirrors `AssistantMessage`'s full-width column so the
 * skeleton reads as an in-progress transcript rather than generic loading chrome.
 */
function SkeletonAssistantRow() {
  return (
    <div className="flex w-full min-w-0 pb-4 pt-1" aria-hidden="true">
      <div className="flex min-w-0 flex-1 flex-col gap-2.5">
        <Skeleton className="h-3.5 w-4/5" />
        <Skeleton className="h-3.5 w-full" />
        <Skeleton className="h-3.5 w-2/3" />
      </div>
    </div>
  );
}

/**
 * Placeholder user row — mirrors `UserMessage`'s right-aligned bubble silhouette.
 */
function SkeletonUserRow() {
  return (
    <div className="flex w-full min-w-0 justify-end pb-4 pt-1" aria-hidden="true">
      <div
        className={cn(
          "flex w-[min(60%,28rem)] flex-col gap-2 rounded-lg px-3 py-transcript-message-y",
          "bg-chat-fill-user"
        )}
      >
        <Skeleton className="h-3.5 w-full" />
        <Skeleton className="h-3.5 w-3/4" />
      </div>
    </div>
  );
}

/**
 * Lightweight message skeleton shown while the transcript fetch is in flight. Built
 * from the `@compozy/ui` `Skeleton` primitive (`animate-shimmer` over neutral surfaces) so
 * a loading session is never mistaken for an empty one.
 */
function ThreadMessageSkeleton() {
  return (
    <div
      role="status"
      aria-label="Loading transcript"
      data-testid="thread-transcript-skeleton"
      className="flex w-full min-w-0 flex-col py-6"
    >
      <SkeletonAssistantRow />
      <SkeletonUserRow />
      <SkeletonAssistantRow />
    </div>
  );
}

/**
 * Empty transcript pane — shown ONLY when the fetch succeeded and nothing has been said
 * yet (zero messages, or only status events that render no row). One plain line; the
 * composer below is the call to action.
 */
function ThreadEmpty({ agentName }: { agentName: string }) {
  return (
    <div className={STATE_PANE_FRAME}>
      <p className="max-w-md text-center text-small-body text-muted">
        Send {agentName} a message to get started.
      </p>
    </div>
  );
}

function ThreadStarting({ agentName }: { agentName: string }) {
  return (
    <Empty
      aria-live="polite"
      className={STATE_PANE_FRAME}
      data-testid="thread-session-starting"
      description="Your session is saved. You can type once it's ready."
      icon={<Spinner aria-hidden="true" className="size-3.75 text-info" />}
      role="status"
      size="compact"
      title={`Getting ${agentName} ready…`}
    />
  );
}

function ThreadStartupFailure({
  agentName,
  failure,
}: {
  agentName: string;
  failure: SessionFailurePayload;
}) {
  const detail = failure.summary?.trim() || "The agent didn't start.";
  return (
    <Empty
      action={
        RUNTIME_RECOVERY_FAILURE_KINDS.has(failure.kind) ? (
          <Button
            nativeButton={false}
            render={
              <Link
                params={{ name: agentName }}
                search={{ section: "runtime" }}
                to="/agents/$name/settings"
              />
            }
            size="sm"
            variant="secondary"
          >
            Check agent settings
          </Button>
        ) : null
      }
      className={STATE_PANE_FRAME}
      data-testid="thread-session-startup-failure"
      description={detail}
      icon={AlertCircle}
      role="alert"
      size="compact"
      title={`${agentName} couldn't start`}
    />
  );
}

/**
 * Retryable error pane — shown when the transcript fetch failed. The raw
 * detail stays behind "Details"; the recovery action refetches the transcript.
 */
function ThreadError({ error, onRetry }: { error: Error | null; onRetry: () => void }) {
  const detail = formatMessageError(error);
  return (
    <Empty
      action={
        <Button
          type="button"
          size="sm"
          variant="secondary"
          onClick={onRetry}
          data-testid="thread-transcript-error-retry"
        >
          Try again
        </Button>
      }
      cause={detail ? <span data-testid="thread-transcript-error-detail">{detail}</span> : null}
      className={STATE_PANE_FRAME}
      data-testid="thread-transcript-error"
      description="Your history is saved. Try again in a moment."
      icon={AlertCircle}
      role="alert"
      size="compact"
      title="Couldn't load this conversation"
    />
  );
}

/**
 * Never loaded, and the live stream gave up (US-018.AC-2): the pane says the
 * conversation didn't sync and how many tries, and offers Try again — it never
 * poses as an empty session. Nothing is lost: the history is saved.
 */
function ThreadSyncFailed({ attempts, onRetry }: { attempts: number; onRetry: () => void }) {
  return (
    <Empty
      action={
        <Button
          type="button"
          size="sm"
          onClick={onRetry}
          variant="secondary"
          data-testid="thread-transcript-sync-failed-retry"
        >
          Try again
        </Button>
      }
      className={STATE_PANE_FRAME}
      data-testid="thread-transcript-sync-failed"
      description={`Couldn't connect after ${attempts} tries. Your history is saved.`}
      icon={WifiOff}
      role="alert"
      size="compact"
      title="This conversation didn't sync"
    />
  );
}

/**
 * State branch for an empty-count transcript: `pending` → skeleton, `error` → retryable
 * pane, a dead live stream → sync-failed pane, `success` → empty-state copy. Extracted so
 * the states are unit-testable in isolation from the assistant-ui runtime that wraps the
 * message rows.
 */
export function ThreadStatePane({
  status,
  agentName,
  error,
  onRetry,
  sessionState,
  failure,
  startupFailed,
  isSessionRunning = false,
  syncFailure = null,
}: {
  status: SessionTranscriptThreadStatus;
  agentName: string;
  error: Error | null;
  onRetry: () => void;
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
  startupFailed?: boolean;
  isSessionRunning?: boolean;
  /** Retries exhausted on the live stream; `attempts` names how many. */
  syncFailure?: { attempts: number; retry: () => void } | null;
}) {
  if (sessionState === "starting") {
    return <ThreadStarting agentName={agentName} />;
  }
  if (startupFailed && failure) {
    return <ThreadStartupFailure agentName={agentName} failure={failure} />;
  }
  if (status === "pending") {
    return <ThreadMessageSkeleton />;
  }
  if (status === "error") {
    return <ThreadError error={error} onRetry={onRetry} />;
  }
  if (syncFailure) {
    return <ThreadSyncFailed attempts={syncFailure.attempts} onRetry={syncFailure.retry} />;
  }
  if (isSessionRunning) {
    // The sole live-status indicator lives between the viewport and composer.
    // Keep this viewport blank until durable transcript content arrives.
    return null;
  }
  return <ThreadEmpty agentName={agentName} />;
}
