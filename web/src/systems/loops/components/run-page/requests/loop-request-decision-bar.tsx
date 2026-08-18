import { useId } from "react";
import { Check, type LucideIcon, MessageSquare, Pencil, X } from "lucide-react";

import { Button, Field, FieldLabel, Textarea } from "@compozy/ui";

import {
  LOOP_REQUEST_DECISION_LABEL,
  type LoopRequestDecision,
} from "../../../lib/loop-request-vocabulary";

export interface LoopRequestDecisionBarProps {
  decisions: readonly LoopRequestDecision[];
  selected: LoopRequestDecision | null;
  disabled?: boolean;
  note: string;
  onNoteChange: (note: string) => void;
  onSelect: (decision: LoopRequestDecision) => void;
}

const DECISION_ICON: Record<LoopRequestDecision, LucideIcon> = {
  approve: Check,
  edit: Pencil,
  reject: X,
  respond: MessageSquare,
};

const REJECT_REST_CLASS = "text-danger hover:border-danger/40 hover:text-danger";

export function LoopRequestDecisionBar({
  decisions,
  selected,
  disabled,
  note,
  onNoteChange,
  onSelect,
}: LoopRequestDecisionBarProps) {
  const noteId = useId();
  if (decisions.length === 0) return null;
  return (
    <div className="flex flex-col gap-3">
      <div aria-label="Decisions" className="flex flex-wrap gap-2" role="group">
        {decisions.map(decision => {
          const Icon = DECISION_ICON[decision];
          const isSelected = selected === decision;
          return (
            <Button
              aria-pressed={isSelected}
              className={decision === "reject" && !isSelected ? REJECT_REST_CLASS : undefined}
              data-testid={`loop-request-decision-${decision}`}
              disabled={disabled}
              key={decision}
              onClick={() => onSelect(decision)}
              size="sm"
              type="button"
              variant={decisionVariant(decision, isSelected)}
            >
              <Icon aria-hidden="true" />
              {LOOP_REQUEST_DECISION_LABEL[decision]}
            </Button>
          );
        })}
      </div>
      {selected ? (
        <Field>
          <FieldLabel htmlFor={noteId}>
            Note <span className="text-muted">optional</span>
          </FieldLabel>
          <Textarea
            data-testid="loop-request-note"
            disabled={disabled}
            id={noteId}
            onChange={event => onNoteChange(event.target.value)}
            rows={2}
            value={note}
          />
        </Field>
      ) : null}
    </div>
  );
}

function decisionVariant(decision: LoopRequestDecision, isSelected: boolean) {
  if (!isSelected) return "outline" as const;
  return decision === "reject" ? ("destructive-solid" as const) : ("primary" as const);
}
