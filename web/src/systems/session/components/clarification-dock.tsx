import type { KeyboardEvent as ReactKeyboardEvent } from "react";

import {
  Button,
  ChoiceFree,
  ChoiceFreeBar,
  ChoiceFreeTextarea,
  ChoiceKey,
  ChoiceList,
  ChoiceRow,
  Dock,
} from "@compozy/ui";

import { useClarificationDock } from "../hooks/use-clarification-dock";
import { formatClarifyDeadline } from "../lib/clarify-event";
import type { ClarificationPending } from "../types";

export interface ClarificationDockProps {
  clarification: ClarificationPending;
  sessionId: string;
  workspaceId: string;
  /** "1/N" when more decisions wait behind this one. */
  countLabel: string | null;
  onResolved: () => void;
}

/**
 * The pending clarification as a composer-docked decision panel. Bounded
 * questions answer as 30px choice rows on digit keys 1–9; a question with no
 * choices renders the quiet free-text form (Enter submits, Shift+Enter breaks).
 * The deadline hint is static — the broker enforces the timeout server-side.
 * Submitting and retryable errors render as quiet dock status lines.
 */
export function ClarificationDock({
  clarification,
  sessionId,
  workspaceId,
  countLabel,
  onResolved,
}: ClarificationDockProps) {
  const dock = useClarificationDock({ clarification, sessionId, workspaceId, onResolved });
  const deadlineHint = formatClarifyDeadline(clarification.deadline);

  const handleTextareaKeyDown = (event: ReactKeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    dock.submitText();
  };

  return (
    <Dock data-testid="clarification-dock" role="region" aria-label="Clarification question">
      <Dock.Head>
        <Dock.Eyebrow data-testid="clarification-dock-eyebrow">Question</Dock.Eyebrow>
        <Dock.Title data-testid="clarification-dock-question">{clarification.question}</Dock.Title>
        {countLabel ? <Dock.Count>{countLabel}</Dock.Count> : null}
        {deadlineHint ? (
          <Dock.Deadline data-testid="clarification-dock-deadline">
            times out {deadlineHint}
          </Dock.Deadline>
        ) : null}
      </Dock.Head>
      {dock.hasChoices ? (
        <ChoiceList aria-label="Answer choices" data-testid="clarification-dock-choices">
          {dock.choices.map((choice, index) => (
            <ChoiceRow
              key={choice}
              data-testid="clarification-dock-choice"
              data-choice-index={index}
              aria-pressed={dock.submittedChoice === index}
              disabled={dock.isSubmitting}
              onClick={() => dock.submitChoice(index)}
            >
              {index < dock.choiceKeyLimit ? <ChoiceKey>{index + 1}</ChoiceKey> : null}
              {choice}
            </ChoiceRow>
          ))}
        </ChoiceList>
      ) : (
        <ChoiceFree data-testid="clarification-dock-free">
          <ChoiceFreeTextarea
            aria-label="Your answer"
            data-testid="clarification-dock-text-input"
            disabled={dock.isSubmitting}
            onChange={event => dock.setText(event.target.value)}
            onKeyDown={handleTextareaKeyDown}
            placeholder="Type your answer…"
            rows={2}
            value={dock.text}
          />
          <ChoiceFreeBar>
            <span className="flex-1" />
            <Button
              size="sm"
              variant="primary"
              data-testid="clarification-dock-submit"
              disabled={!dock.canSubmitText}
              onClick={dock.submitText}
            >
              Send answer
            </Button>
          </ChoiceFreeBar>
        </ChoiceFree>
      )}
      {dock.isSubmitting ? (
        <Dock.Status role="status" data-testid="clarification-dock-status">
          Sending your answer…
        </Dock.Status>
      ) : null}
      {dock.errorMessage ? (
        <Dock.Status tone="danger" role="alert" data-testid="clarification-dock-error">
          {dock.errorMessage}
        </Dock.Status>
      ) : null}
    </Dock>
  );
}
