import { type FormEvent, useId, useState } from "react";

import { Button, Eyebrow, Field, FieldLabel, Spinner, StatusDot, Textarea } from "@agh/ui";

export interface ClarificationCardProps {
  question: string;
  /** Live choices; empty/undefined renders the free-text form. */
  choices: string[] | undefined;
  /** Static, non-ticking deadline hint (absolute local time), or null when absent. */
  deadlineHint: string | null;
  isSubmitting: boolean;
  /** Retryable failure message; controls stay enabled so a resubmit retries. */
  errorMessage: string | null;
  onAnswerChoice: (index: number) => void;
  onAnswerText: (text: string) => void;
}

/**
 * Presentational clarification "question" card. Renders the operator's answer surface — bounded
 * choice buttons or a free-text form — from live data supplied by its container. Deliberately not a
 * permission prompt: inline (non-sticky), a single accent signal, distinct copy.
 */
export function ClarificationCard({
  question,
  choices,
  deadlineHint,
  isSubmitting,
  errorMessage,
  onAnswerChoice,
  onAnswerText,
}: ClarificationCardProps) {
  const hasChoices = Array.isArray(choices) && choices.length > 0;
  return (
    <div
      aria-label="Clarification question"
      className="my-1.5 flex w-full min-w-0 flex-col gap-2 rounded-md border border-line bg-canvas-soft px-3 py-2.5"
      data-state={isSubmitting ? "submitting" : "pending"}
      data-testid="clarification-card"
      role="group"
    >
      <header className="flex flex-col gap-1">
        <span className="flex items-center gap-1.5">
          <StatusDot tone="accent" />
          <Eyebrow data-testid="clarification-eyebrow">Question</Eyebrow>
        </span>
        <p
          className="text-small-body leading-snug text-fg-strong"
          data-testid="clarification-question"
        >
          {question}
        </p>
      </header>
      {hasChoices ? (
        <ClarificationChoices
          choices={choices}
          disabled={isSubmitting}
          onAnswerChoice={onAnswerChoice}
        />
      ) : (
        <ClarificationTextForm disabled={isSubmitting} onAnswerText={onAnswerText} />
      )}
      {deadlineHint ? (
        <p className="text-form-hint text-muted" data-testid="clarification-deadline">
          Times out at {deadlineHint}
        </p>
      ) : null}
      {isSubmitting ? (
        <p
          className="flex items-center gap-1.5 text-form-hint text-subtle"
          data-testid="clarification-pending-status"
          role="status"
        >
          <Spinner aria-hidden="true" className="size-3" />
          Sending your answer…
        </p>
      ) : null}
      {errorMessage ? (
        <p className="text-form-hint text-danger" data-testid="clarification-error" role="alert">
          {errorMessage}
        </p>
      ) : null}
    </div>
  );
}

interface ClarificationChoicesProps {
  choices: string[];
  disabled: boolean;
  onAnswerChoice: (index: number) => void;
}

function ClarificationChoices({ choices, disabled, onAnswerChoice }: ClarificationChoicesProps) {
  return (
    <div
      aria-label="Answer choices"
      className="flex flex-wrap items-center gap-1.5"
      data-testid="clarification-choices"
      role="group"
    >
      {choices.map((choice, index) => (
        <Button
          key={choice}
          data-choice-index={index}
          data-testid="clarification-choice"
          disabled={disabled}
          onClick={() => onAnswerChoice(index)}
          size="sm"
          variant="outline"
        >
          {choice}
        </Button>
      ))}
    </div>
  );
}

interface ClarificationTextFormProps {
  disabled: boolean;
  onAnswerText: (text: string) => void;
}

function ClarificationTextForm({ disabled, onAnswerText }: ClarificationTextFormProps) {
  const inputId = useId();
  const [text, setText] = useState("");
  const trimmed = text.trim();
  const canSubmit = !disabled && trimmed.length > 0;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }
    onAnswerText(trimmed);
  };

  return (
    <form
      className="flex flex-col gap-1.5"
      data-testid="clarification-text-form"
      onSubmit={handleSubmit}
    >
      <Field>
        <FieldLabel className="sr-only" htmlFor={inputId}>
          Your answer
        </FieldLabel>
        <Textarea
          aria-label="Your answer"
          data-testid="clarification-text-input"
          disabled={disabled}
          id={inputId}
          onChange={event => setText(event.target.value)}
          placeholder="Type your answer…"
          rows={2}
          value={text}
        />
      </Field>
      <div className="flex items-center justify-end">
        <Button data-testid="clarification-submit" disabled={!canSubmit} size="sm" type="submit">
          {disabled ? <Spinner aria-hidden="true" className="size-3" /> : null}
          Send answer
        </Button>
      </div>
    </form>
  );
}
