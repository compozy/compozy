import { Link } from "@tanstack/react-router";
import { WifiOff } from "lucide-react";

import { Button, Eyebrow, Skeleton, Spinner } from "@compozy/ui";

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
          "flex w-[min(60%,28rem)] flex-col gap-2 rounded-xl border px-4 py-3",
          "border-line bg-canvas-soft"
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
 * Empty transcript pane — shown ONLY when the fetch succeeded with zero messages.
 */
function ThreadEmpty({ agentName }: { agentName: string }) {
  return (
    <div className={STATE_PANE_FRAME}>
      <div className="max-w-md text-center">
        <Eyebrow className="text-subtle">{agentName}</Eyebrow>
        <p className="mt-2 text-small-body text-muted">
          Start the conversation. Everything you and the agent do here is saved.
        </p>
      </div>
    </div>
  );
}

function ThreadStarting({ agentName }: { agentName: string }) {
  return (
    <div className={STATE_PANE_FRAME}>
      <div
        className="flex max-w-md flex-col items-center text-center"
        role="status"
        aria-live="polite"
        data-testid="thread-session-starting"
      >
        <Spinner className="size-5 text-info" aria-hidden="true" />
        <Eyebrow className="mt-3 text-info">Starting session</Eyebrow>
        <p className="mt-2 text-small-body text-muted">
          The session is saved. CompozyOS is preparing {agentName} and will enable the composer when
          the runtime is active.
        </p>
      </div>
    </div>
  );
}

function ThreadStartupFailure({
  agentName,
  failure,
}: {
  agentName: string;
  failure: SessionFailurePayload;
}) {
  const detail = failure.summary?.trim() || "The session runtime did not become active.";
  return (
    <div className={STATE_PANE_FRAME}>
      <div
        className="flex max-w-md flex-col items-center text-center"
        role="alert"
        data-testid="thread-session-startup-failure"
      >
        <Eyebrow className="text-danger">Session failed to start</Eyebrow>
        <p className="mt-2 text-small-body text-muted">{detail}</p>
        {RUNTIME_RECOVERY_FAILURE_KINDS.has(failure.kind) ? (
          <Button
            className="mt-4"
            nativeButton={false}
            render={
              <Link
                params={{ name: agentName }}
                search={{ section: "runtime" }}
                to="/agents/$name/settings"
              />
            }
            size="sm"
            variant="outline"
          >
            Review agent runtime
          </Button>
        ) : null}
      </div>
    </div>
  );
}

/**
 * Retryable error pane — shown when the transcript fetch failed. Surfaces the provider
 * detail when present and keeps the recovery action wired to the transcript refetch.
 */
function ThreadError({ error, onRetry }: { error: Error | null; onRetry: () => void }) {
  const detail = formatMessageError(error);
  return (
    <div className={STATE_PANE_FRAME}>
      <div className="max-w-md text-center" role="alert" data-testid="thread-transcript-error">
        <Eyebrow className="text-danger">Couldn&apos;t load this conversation</Eyebrow>
        <p className="mt-2 text-small-body text-muted">
          {detail ? (
            <span data-testid="thread-transcript-error-detail">{detail}</span>
          ) : (
            "The transcript could not be loaded."
          )}
        </p>
        <Button
          type="button"
          variant="outline"
          className="mt-4"
          onClick={onRetry}
          data-testid="thread-transcript-error-retry"
        >
          Retry transcript
        </Button>
      </div>
    </div>
  );
}

/**
 * Never loaded, and the live stream gave up (US-018.AC-2): the pane says the
 * conversation didn't sync and how many tries, and offers Try again — it never
 * poses as an empty session. Nothing is lost: the history is saved.
 */
function ThreadSyncFailed({ attempts, onRetry }: { attempts: number; onRetry: () => void }) {
  return (
    <div className={STATE_PANE_FRAME}>
      <div
        className="flex max-w-md flex-col items-center gap-2.5 text-center"
        role="alert"
        data-testid="thread-transcript-sync-failed"
      >
        <span
          aria-hidden="true"
          className="grid size-8.5 place-items-center rounded-md bg-canvas-soft text-subtle"
        >
          <WifiOff className="size-4" />
        </span>
        <p className="text-small-body font-medium text-fg-strong">
          This conversation didn&apos;t sync
        </p>
        <p className="max-w-[44ch] text-eyebrow text-muted text-pretty">
          Couldn&apos;t reach CompozyOS after {attempts} tries. Nothing here is lost — the history
          is saved.
        </p>
        <Button
          type="button"
          size="sm"
          className="mt-1"
          onClick={onRetry}
          variant="outline"
          data-testid="thread-transcript-sync-failed-retry"
        >
          Try again
        </Button>
      </div>
    </div>
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
