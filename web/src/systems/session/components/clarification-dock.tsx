import { ChevronDown, ChevronUp, MessageCircleQuestion } from "lucide-react";
import { useState, type KeyboardEvent as ReactKeyboardEvent } from "react";

import {
  Button,
  ChoiceFree,
  ChoiceFreeBar,
  ChoiceFreeTextarea,
  ChoiceKey,
  ChoiceList,
  ChoiceRow,
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Dock,
  KindIcon,
} from "@compozy/ui";

import { useClarificationDock } from "../hooks/use-clarification-dock";
import { formatClarifyDeadline } from "../lib/clarify-event";
import type { ClarificationPending } from "../types";

export interface ClarificationDockProps {
  enabled?: boolean;
  clarification: ClarificationPending;
  sessionId: string;
  workspaceId: string;
  /** "1/N" when more decisions wait behind this one. */
  countLabel: string | null;
  onResolved: () => void;
}

function indexedChoices(choices: readonly string[]) {
  const occurrences = new Map<string, number>();
  return choices.map((choice, index) => {
    const occurrence = (occurrences.get(choice) ?? 0) + 1;
    occurrences.set(choice, occurrence);
    return { choice, index, key: `${choice}\u0000${occurrence}` };
  });
}

/** First non-blank line — the collapsed dock's one-line summary of the question. */
function questionSummary(question: string): string {
  return question.split("\n").find(line => line.trim().length > 0) ?? question;
}

/**
 * The pending clarification as a composer-docked decision card led by its
 * identity well. Bounded
 * questions answer as 30px choice rows on digit keys 1–9; a question with no
 * choices renders the quiet free-text form (Enter submits, Shift+Enter breaks).
 * The deadline hint is static — the broker enforces the timeout server-side.
 * Submitting and retryable errors render as quiet dock status lines.
 *
 * The question keeps the agent's line breaks and scrolls inside a dock bounded
 * to half the viewport. The head chevron collapses the dock to the question's
 * first line; it always mounts expanded, and the host keys it by request id,
 * so a collapse lasts only until that question resolves. Digit keys still
 * answer while collapsed.
 */
export function ClarificationDock({
  enabled = true,
  clarification,
  sessionId,
  workspaceId,
  countLabel,
  onResolved,
}: ClarificationDockProps) {
  const dock = useClarificationDock({
    clarification,
    enabled,
    sessionId,
    workspaceId,
    onResolved,
  });
  const deadlineHint = formatClarifyDeadline(clarification.deadline);

  const handleTextareaKeyDown = (event: ReactKeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    dock.submitText();
  };

  const [expanded, setExpanded] = useState(true);

  return (
    <Dock
      data-testid="clarification-dock"
      data-expanded={expanded}
      role="region"
      aria-label="Clarification question"
      className="@container max-h-[50dvh]"
    >
      <Collapsible className="contents" open={expanded} onOpenChange={setExpanded}>
        <Dock.Head
          className={cn(
            "grid min-h-0 grid-cols-[auto_minmax(0,1fr)_auto] grid-rows-[minmax(0,1fr)_auto] items-start gap-x-2.5 gap-y-1",
            "@md:grid-cols-[auto_minmax(0,1fr)_auto_auto] @md:grid-rows-[minmax(0,1fr)]"
          )}
        >
          <KindIcon
            data-testid="clarification-dock-well"
            icon={MessageCircleQuestion}
            tone="well"
            className="col-start-1 row-start-1"
          />
          {expanded ? (
            <Dock.Title
              multiline
              data-testid="clarification-dock-question"
              className="col-start-2 row-start-1 min-h-0 self-stretch overflow-y-auto py-1"
            >
              {clarification.question}
            </Dock.Title>
          ) : (
            <Dock.Title
              data-testid="clarification-dock-question"
              className="col-start-2 row-start-1 min-w-0 truncate py-1"
            >
              {questionSummary(clarification.question)}
            </Dock.Title>
          )}
          {countLabel || deadlineHint ? (
            <div className="col-start-2 row-start-2 flex items-center gap-2.5 @md:col-start-3 @md:row-start-1 @md:h-6.5">
              {countLabel ? <Dock.Count>{countLabel}</Dock.Count> : null}
              {deadlineHint ? (
                <Dock.Deadline data-testid="clarification-dock-deadline" className="ml-0">
                  times out {deadlineHint}
                </Dock.Deadline>
              ) : null}
            </div>
          ) : null}
          <div className="col-start-3 row-start-1 flex h-6.5 items-center @md:col-start-4">
            <CollapsibleTrigger
              aria-label={expanded ? "Collapse question" : "Expand question"}
              data-testid="clarification-dock-toggle"
              render={<Button variant="ghost" size="icon-xs" />}
            >
              {expanded ? <ChevronDown aria-hidden="true" /> : <ChevronUp aria-hidden="true" />}
            </CollapsibleTrigger>
          </div>
        </Dock.Head>
        <CollapsibleContent className="shrink-0">
          {dock.hasChoices ? (
            <ChoiceList aria-label="Answer choices" data-testid="clarification-dock-choices">
              {indexedChoices(dock.choices).map(({ choice, index, key }) => (
                <ChoiceRow
                  key={key}
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
                  variant="primary"
                  data-testid="clarification-dock-submit"
                  disabled={!dock.canSubmitText}
                  onClick={dock.submitText}
                  kbd="↵"
                >
                  Send answer
                </Button>
              </ChoiceFreeBar>
            </ChoiceFree>
          )}
        </CollapsibleContent>
      </Collapsible>
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
