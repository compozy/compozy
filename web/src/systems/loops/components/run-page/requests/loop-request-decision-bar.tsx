import { useId } from "react";
import { Check, type LucideIcon, MessageSquare, Pencil, X } from "lucide-react";

import { Field, FieldLabel, RadioCard, Textarea } from "@compozy/ui";

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
      <div aria-label="Decision" className="flex flex-col gap-1.5" role="radiogroup">
        {decisions.map(decision => {
          const isSelected = selected === decision;
          return (
            <RadioCard
              className={isSelected ? undefined : "bg-canvas-tint"}
              data-testid={`loop-request-decision-${decision}`}
              disabled={disabled}
              icon={DECISION_ICON[decision]}
              key={decision}
              onSelect={() => onSelect(decision)}
              selected={isSelected}
              title={LOOP_REQUEST_DECISION_LABEL[decision]}
            />
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
