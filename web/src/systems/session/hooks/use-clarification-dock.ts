import { useEffect, useEffectEvent, useState } from "react";

import { ClarificationNotAnswerableError, SessionApiError } from "../adapters/session-api";
import { isEditableTarget } from "../lib/editable-target";
import type { AnswerClarificationBody, ClarificationPending } from "../types";
import { useAnswerSessionClarification } from "./use-session-clarifications";

const CHOICE_KEY_LIMIT = 9;

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

export interface UseClarificationDockOptions {
  clarification: ClarificationPending;
  sessionId: string;
  workspaceId: string;
  onResolved: () => void;
}

/**
 * Behavior for the composer clarification dock: answer submission with
 * terminal-gone handling, bounded-choice digit keys 1–9 (ignoring focused
 * inputs), and the free-text draft state.
 */
export function useClarificationDock({
  clarification,
  sessionId,
  workspaceId,
  onResolved,
}: UseClarificationDockOptions) {
  const answerMutation = useAnswerSessionClarification(workspaceId, sessionId);
  const [text, setText] = useState("");
  const [submittedChoice, setSubmittedChoice] = useState<number | null>(null);
  const requestId = clarification.request_id;
  const choices = clarification.choices ?? [];
  const hasChoices = choices.length > 0;
  const isSubmitting = answerMutation.isPending;

  const submit = (body: AnswerClarificationBody) => {
    if (isSubmitting) {
      return;
    }
    answerMutation.mutate(
      { requestId, body },
      {
        onSuccess: () => onResolved(),
        onError: error => {
          setSubmittedChoice(null);
          if (isTerminalGone(error)) {
            onResolved();
          }
        },
      }
    );
  };

  const submitChoice = (index: number) => {
    setSubmittedChoice(index);
    submit({ choice_index: index });
  };

  const trimmed = text.trim();
  const submitText = () => {
    if (trimmed.length === 0) return;
    submit({ text: trimmed });
  };

  const handleChoiceKey = useEffectEvent((event: KeyboardEvent) => {
    if (!hasChoices || isSubmitting) return;
    if (isEditableTarget(event.target)) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    const index = Number(event.key) - 1;
    const keyedChoices = Math.min(choices.length, CHOICE_KEY_LIMIT);
    if (!Number.isInteger(index) || index < 0 || index >= keyedChoices) {
      return;
    }
    event.preventDefault();
    submitChoice(index);
  });

  useEffect(() => {
    document.addEventListener("keydown", handleChoiceKey);
    return () => document.removeEventListener("keydown", handleChoiceKey);
  }, []);

  return {
    choices,
    hasChoices,
    choiceKeyLimit: CHOICE_KEY_LIMIT,
    isSubmitting,
    errorMessage: retryableErrorMessage(answerMutation.error),
    submittedChoice,
    submitChoice,
    text,
    setText,
    canSubmitText: !isSubmitting && trimmed.length > 0,
    submitText,
  };
}
