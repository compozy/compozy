import {
  Field,
  FieldDescription,
  FieldHeader,
  FieldLabel,
  Input,
  RequiredMark,
  Switch,
  ToggleGroup,
  ToggleGroupItem,
} from "@compozy/ui";

import type { NodeFieldEdit } from "../../lib/loop-editor-draft";
import {
  LOOP_STRATEGY_KINDS,
  type LoopStrategyKind,
  type StrategyFieldSpec,
} from "../../lib/loop-node-schema-types";
import { strategyEdits } from "../../lib/loop-node-strategy-fields";

export interface LoopEditorStrategyFieldProps {
  spec: StrategyFieldSpec;
  disabled?: boolean;
  onChangeFields: (edits: NodeFieldEdit[]) => void;
}

interface StrategyDraft {
  kind: LoopStrategyKind | null;
  threshold: string;
  missingAcceptable: boolean;
}

function asStrategyKind(values: readonly string[]): LoopStrategyKind | null {
  return LOOP_STRATEGY_KINDS.find(kind => values.includes(kind)) ?? null;
}

export function LoopEditorStrategyField({
  spec,
  disabled = false,
  onChangeFields,
}: LoopEditorStrategyFieldProps) {
  function apply(patch: Partial<StrategyDraft>) {
    onChangeFields(
      strategyEdits({
        kind: spec.kind,
        threshold: spec.threshold,
        missingAcceptable: spec.missingAcceptable,
        ...patch,
      })
    );
  }

  return (
    <Field data-slot="loop-node-strategy-field">
      <FieldHeader>
        <FieldLabel>{spec.label}</FieldLabel>
      </FieldHeader>
      <div className="flex flex-col gap-2.5" data-testid="loop-editor-strategy">
        <ToggleGroup
          aria-label={spec.label}
          className="flex-wrap"
          disabled={disabled}
          onValueChange={next => apply({ kind: asStrategyKind(next) })}
          spacing={1.5}
          value={spec.kind ? [spec.kind] : []}
        >
          {LOOP_STRATEGY_KINDS.map(kind => (
            <ToggleGroupItem
              className="font-mono"
              data-testid={`loop-strategy-kind-${kind}`}
              key={kind}
              value={kind}
              variant="outline"
            >
              {kind}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        {spec.kind ? <StrategyRefinements spec={spec} disabled={disabled} onApply={apply} /> : null}
      </div>
      {spec.hint ? <FieldDescription>{spec.hint}</FieldDescription> : null}
    </Field>
  );
}

interface StrategyRefinementsProps {
  spec: StrategyFieldSpec;
  disabled: boolean;
  onApply: (patch: Partial<StrategyDraft>) => void;
}

function StrategyRefinements({ spec, disabled, onApply }: StrategyRefinementsProps) {
  const bestEffort = spec.kind === "best_effort";
  return (
    <>
      {bestEffort || spec.threshold !== "" ? (
        <Field>
          <FieldHeader>
            <FieldLabel>threshold</FieldLabel>
            {bestEffort ? <RequiredMark>*</RequiredMark> : null}
          </FieldHeader>
          <Input
            aria-label="Strategy threshold"
            className="h-8 w-28 px-2.5 font-mono text-form-input"
            data-testid="loop-strategy-threshold"
            disabled={disabled}
            onChange={event => onApply({ threshold: event.target.value })}
            placeholder="66%"
            type="text"
            value={spec.threshold}
          />
          <FieldDescription>A share of the lanes like 66%, or a count like 3.</FieldDescription>
        </Field>
      ) : null}
      <div className="flex items-start gap-3 rounded-md border border-line-soft bg-canvas-soft px-3 py-2.5">
        <Switch
          aria-label="Missing lanes are acceptable"
          checked={spec.missingAcceptable}
          className="mt-0.5"
          data-testid="loop-strategy-missing"
          disabled={disabled}
          onCheckedChange={next => onApply({ missingAcceptable: next })}
        />
        <span className="min-w-0">
          <span className="block text-form-label font-medium text-fg-strong">
            Missing lanes are acceptable
          </span>
          {bestEffort ? (
            <span className="text-form-hint text-subtle">
              Required for best_effort — it acknowledges that some lanes may never materialize.
            </span>
          ) : null}
        </span>
      </div>
    </>
  );
}
