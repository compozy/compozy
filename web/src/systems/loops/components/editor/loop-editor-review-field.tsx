import {
  Field,
  FieldDescription,
  FieldHeader,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  Switch,
  Textarea,
  ToggleGroup,
  ToggleGroupItem,
} from "@compozy/ui";

import type { NodeFieldEdit } from "../../lib/loop-editor-draft";
import { reviewEdits, type ReviewDraft } from "../../lib/loop-node-review-fields";
import {
  LOOP_REVIEW_DECISIONS,
  type LoopReviewDecision,
  type ReviewFieldSpec,
} from "../../lib/loop-node-schema-types";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import { LoopReferenceInput } from "./loop-reference-input";

export interface LoopEditorReviewFieldProps {
  spec: ReviewFieldSpec;
  disabled?: boolean;
  onChangeFields: (edits: NodeFieldEdit[]) => void;
  suggestions: readonly LoopReferenceSuggestion[];
}

const CONTROL_CLASS = "h-8 px-2.5 font-mono text-form-input";

function asDecisions(values: readonly string[]): LoopReviewDecision[] {
  return LOOP_REVIEW_DECISIONS.filter(decision => values.includes(decision));
}

export function LoopEditorReviewField({
  spec,
  disabled = false,
  onChangeFields,
  suggestions,
}: LoopEditorReviewFieldProps) {
  function apply(patch: Partial<ReviewDraft>) {
    onChangeFields(
      reviewEdits({
        enabled: spec.enabled,
        decisions: spec.decisions,
        when: spec.when,
        prompt: spec.prompt,
        agentsAllowed: spec.agentsAllowed,
        onRejectRoute: spec.onRejectRoute,
        expiresAfter: spec.expiresAfter,
        ...patch,
      })
    );
  }

  return (
    <Field className="gap-3" data-slot="loop-node-review-field" data-testid="loop-editor-review">
      <div className="flex items-center gap-3 rounded-md border border-line-soft bg-canvas-soft px-3 py-2.5">
        <Switch
          aria-label={spec.label}
          checked={spec.enabled}
          data-testid="loop-review-enabled"
          disabled={disabled}
          onCheckedChange={next => apply({ enabled: next })}
        />
        <span className="block text-form-label font-medium text-fg-strong">{spec.label}</span>
      </div>
      {spec.enabled ? (
        <>
          <ReviewRequestFields
            spec={spec}
            disabled={disabled}
            onApply={apply}
            suggestions={suggestions}
          />
          <ReviewOutcomeFields spec={spec} disabled={disabled} onApply={apply} />
        </>
      ) : null}
      {spec.hint ? <FieldDescription>{spec.hint}</FieldDescription> : null}
    </Field>
  );
}

interface ReviewSectionProps {
  spec: ReviewFieldSpec;
  disabled: boolean;
  onApply: (patch: Partial<ReviewDraft>) => void;
}

function ReviewRequestFields({
  spec,
  disabled,
  onApply,
  suggestions,
}: ReviewSectionProps & { suggestions: readonly LoopReferenceSuggestion[] }) {
  return (
    <>
      <Field>
        <FieldHeader>
          <FieldLabel>decisions</FieldLabel>
        </FieldHeader>
        <ToggleGroup
          aria-label="Review decisions"
          className="flex-wrap"
          disabled={disabled}
          multiple
          onValueChange={next => onApply({ decisions: asDecisions(next) })}
          spacing={1.5}
          value={spec.decisions}
        >
          {LOOP_REVIEW_DECISIONS.map(decision => (
            <ToggleGroupItem
              className="font-mono"
              data-testid={`loop-review-decision-${decision}`}
              key={decision}
              value={decision}
              variant="outline"
            >
              {decision}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        <FieldDescription>No selection means approve and reject.</FieldDescription>
      </Field>
      <Field>
        <FieldHeader>
          <FieldLabel>when</FieldLabel>
        </FieldHeader>
        <CelConditionInput
          ariaLabel="Review when"
          disabled={disabled}
          onChange={next => onApply({ when: next })}
          placeholder="true"
          suggestions={suggestions}
          testId="loop-review-when"
          value={spec.when}
        />
        <FieldDescription>A CEL condition that decides whether the review opens.</FieldDescription>
      </Field>
      <Field>
        <FieldHeader>
          <FieldLabel>prompt</FieldLabel>
        </FieldHeader>
        <Textarea
          aria-label="Review prompt"
          className="min-h-18.5 resize-y text-form-input leading-relaxed"
          data-testid="loop-review-prompt"
          disabled={disabled}
          onChange={event => onApply({ prompt: event.target.value })}
          placeholder="Apply the migration as proposed?"
          value={spec.prompt}
        />
      </Field>
      <div className="flex items-center gap-3 rounded-md border border-line-soft bg-canvas-soft px-3 py-2.5">
        <Switch
          aria-label="Agents may respond"
          checked={spec.agentsAllowed}
          data-testid="loop-review-agents"
          disabled={disabled}
          onCheckedChange={next => onApply({ agentsAllowed: next })}
        />
        <span className="block text-form-label font-medium text-fg-strong">Agents may respond</span>
      </div>
    </>
  );
}

interface CelConditionInputProps {
  ariaLabel: string;
  disabled: boolean;
  onChange: (next: string) => void;
  placeholder: string;
  suggestions: readonly LoopReferenceSuggestion[];
  testId: string;
  value: string;
}

function CelConditionInput({
  ariaLabel,
  disabled,
  onChange,
  placeholder,
  suggestions,
  testId,
  value,
}: CelConditionInputProps) {
  if (suggestions.length === 0) {
    return (
      <Input
        aria-label={ariaLabel}
        className={CONTROL_CLASS}
        data-testid={testId}
        disabled={disabled}
        onChange={event => onChange(event.target.value)}
        placeholder={placeholder}
        type="text"
        value={value}
      />
    );
  }
  return (
    <LoopReferenceInput
      ariaLabel={ariaLabel}
      cel
      disabled={disabled}
      mono
      onChange={onChange}
      placeholder={placeholder}
      suggestions={suggestions}
      testId={testId}
      value={value}
    />
  );
}

function ReviewOutcomeFields({ spec, disabled, onApply }: ReviewSectionProps) {
  const stale = spec.onRejectRoute !== "" && !spec.targets.includes(spec.onRejectRoute);
  return (
    <>
      <Field>
        <FieldHeader>
          <FieldLabel>on_reject.route</FieldLabel>
        </FieldHeader>
        <NativeSelect
          aria-label="On reject route"
          data-testid="loop-review-on-reject"
          disabled={disabled}
          onChange={event => onApply({ onRejectRoute: event.target.value })}
          value={spec.onRejectRoute}
        >
          <NativeSelectOption value="">
            {spec.targets.length === 0 ? "No forward nodes" : "Not set"}
          </NativeSelectOption>
          {stale ? (
            <NativeSelectOption value={spec.onRejectRoute}>
              {spec.onRejectRoute} (not forward)
            </NativeSelectOption>
          ) : null}
          {spec.targets.map(target => (
            <NativeSelectOption key={target} value={target}>
              {target}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <FieldDescription>
          Where a rejected run continues. Forward nodes only — a backward target is
          error_route_backward.
        </FieldDescription>
      </Field>
      <Field>
        <FieldHeader>
          <FieldLabel>expires.after</FieldLabel>
        </FieldHeader>
        <Input
          aria-label="Review expires after"
          className={`${CONTROL_CLASS} w-28`}
          data-testid="loop-review-expires"
          disabled={disabled}
          onChange={event => onApply({ expiresAfter: event.target.value })}
          placeholder="24h"
          type="text"
          value={spec.expiresAfter}
        />
      </Field>
    </>
  );
}
