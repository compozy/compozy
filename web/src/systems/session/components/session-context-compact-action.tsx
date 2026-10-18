import { useEffect } from "react";
import { Button, FieldError, Spinner } from "@compozy/ui";

import { useCompactSession } from "../hooks/use-compact-session";
import {
  canCompactNow,
  describeCompactionFailure,
  resolveCompactionCommand,
} from "../lib/session-compaction";
import { hasActivePrompt } from "../lib/session-running";
import type { SessionPayload } from "../types";

interface SessionContextCompactViewProps {
  /** A turn is running or the session cannot take a request; the button stays but is inert. */
  disabled: boolean;
  /** The inline sentence for the last refused or failed request. */
  error: string | null;
  onCompact: () => void;
  /** The request is in flight. */
  pending: boolean;
}

/** The Compact now button and its inline refusal, as one action row of the context rail. */
function SessionContextCompactView({
  disabled,
  error,
  onCompact,
  pending,
}: SessionContextCompactViewProps) {
  return (
    <div className="flex flex-col items-start gap-1.5">
      <Button
        aria-busy={pending}
        data-testid="session-context-compact-now"
        disabled={disabled || pending}
        onClick={onCompact}
        size="sm"
        type="button"
        variant="secondary"
      >
        {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
        Compact now
      </Button>
      {error ? <FieldError data-testid="session-context-compact-error">{error}</FieldError> : null}
    </div>
  );
}

function SessionContextCompactControl({
  session,
  workspaceId,
}: {
  session: SessionPayload;
  workspaceId: string;
}) {
  const running = hasActivePrompt(session);
  const { error, isPending, mutate, reset } = useCompactSession(workspaceId, session.id);

  // A refusal stays readable while the session is busy; once the turn ends it no longer describes anything.
  useEffect(() => {
    if (!running) reset();
  }, [reset, running]);

  return (
    <SessionContextCompactView
      disabled={!canCompactNow(session)}
      error={error ? describeCompactionFailure(error) : null}
      onCompact={() => mutate()}
      pending={isPending}
    />
  );
}

/**
 * Asks the agent to compact its own context. Offered only when the session
 * advertises a compaction command and has a workspace to address the request
 * to; it is the agent's command, so nothing here decides what compaction means.
 */
export function SessionContextCompactAction({ session }: { session: SessionPayload }) {
  const workspaceId = session.workspace_id?.trim();
  if (!workspaceId || resolveCompactionCommand(session.available_commands) === null) return null;
  return (
    <SessionContextCompactControl key={session.id} session={session} workspaceId={workspaceId} />
  );
}
