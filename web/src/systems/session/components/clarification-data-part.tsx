import { AlertTriangle, RefreshCw } from "lucide-react";
import { useState } from "react";

import { Alert, AlertActions, AlertDescription, AlertTitle, Button, Spinner } from "@agh/ui";

import { ClarificationNotAnswerableError, SessionApiError } from "../adapters/session-api";
import {
  useAnswerSessionClarification,
  useSessionClarifications,
} from "../hooks/use-session-clarifications";
import { formatClarifyDeadline, parseClarifyEvent } from "../lib/clarify-event";
import type { AgentEventPayload, AnswerClarificationBody } from "../types";
import { ClarificationCard } from "./clarification-card";
import { ClarificationReceipt } from "./clarification-receipt";

export interface ClarificationDataPartProps {
  data: AgentEventPayload;
  sessionId: string;
  workspaceId: string;
}

/**
 * Timeline dispatch for a `clarify` data-agh-event. Parses the durable payload and branches by
 * status: a pending event drives the live-gated interactive card; a terminal event renders a
 * historical receipt. This dispatcher owns no query hooks, so terminal rows never subscribe.
 */
export function ClarificationDataPart({
  data,
  sessionId,
  workspaceId,
}: ClarificationDataPartProps) {
  const view = parseClarifyEvent(data);
  if (!view) {
    return null;
  }
  if (view.status === "pending") {
    return (
      <PendingClarification
        question={view.request.question}
        requestId={view.requestId}
        sessionId={sessionId}
        workspaceId={workspaceId}
      />
    );
  }
  return <ClarificationReceipt view={view} />;
}

function isTerminalGone(error: unknown): boolean {
  if (error instanceof ClarificationNotAnswerableError) {
    return true;
  }
  return error instanceof SessionApiError && error.status === 404;
}

function retryableErrorMessage(error: unknown): string | null {
  if (error == null || isTerminalGone(error)) {
    return null;
  }
  // Neutral copy: retryable causes span 400 (invalid), 503 (broker down), and network, so the
  // message cannot claim a single cause. Controls stay enabled, so a resubmit is the retry.
  return "Couldn't send your answer. Try again.";
}

interface PendingClarificationProps {
  question: string;
  requestId: string;
  sessionId: string;
  workspaceId: string;
}

/**
 * Live-gated pending surface. Answer controls render only while the exact-authority GET confirms the
 * request; a failed read preserves the durable question with a retry, while a confirmed-gone request
 * renders nothing. Hides locally after submit success or a terminal race instead of awaiting refetch.
 */
function PendingClarification({
  question,
  requestId,
  sessionId,
  workspaceId,
}: PendingClarificationProps) {
  const [answered, setAnswered] = useState(false);
  const clarifications = useSessionClarifications(workspaceId, sessionId);
  const answerMutation = useAnswerSessionClarification(workspaceId, sessionId);

  if (answered) {
    return null;
  }
  if (clarifications.isError) {
    return (
      <Alert className="w-full" data-testid="clarification-load-error" variant="warning">
        <AlertTriangle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
        <AlertTitle>Clarification unavailable</AlertTitle>
        <AlertDescription>
          <p>{question}</p>
          <p>
            Couldn't confirm whether this clarification is still pending. Retry before answering.
          </p>
        </AlertDescription>
        <AlertActions>
          <Button
            disabled={clarifications.isFetching}
            onClick={() => void clarifications.refetch()}
            size="sm"
            type="button"
            variant="neutral"
          >
            {clarifications.isFetching ? (
              <Spinner className="size-3" />
            ) : (
              <RefreshCw aria-hidden="true" className="size-3" />
            )}
            Retry clarification
          </Button>
        </AlertActions>
      </Alert>
    );
  }

  const live = clarifications.data?.find(entry => entry.request_id === requestId);
  if (!live) {
    return null;
  }

  const submit = (body: AnswerClarificationBody) => {
    if (answerMutation.isPending) {
      return;
    }
    answerMutation.mutate(
      { requestId, body },
      {
        onSuccess: () => setAnswered(true),
        onError: error => {
          if (isTerminalGone(error)) {
            setAnswered(true);
          }
        },
      }
    );
  };

  return (
    <ClarificationCard
      choices={live.choices}
      deadlineHint={formatClarifyDeadline(live.deadline)}
      errorMessage={retryableErrorMessage(answerMutation.error)}
      isSubmitting={answerMutation.isPending}
      onAnswerChoice={index => submit({ choice_index: index })}
      onAnswerText={text => submit({ text })}
      question={live.question}
    />
  );
}
