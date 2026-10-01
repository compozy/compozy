import { Gauge } from "lucide-react";

import { Input, NativeSelect, NativeSelectOption } from "@compozy/ui";

import {
  buildOverrideFields,
  clampOverrideValue,
  summarizeRunLimits,
  type LoopBudgetPolicy,
  type LoopOverrideDraft,
  type LoopOverrideField,
} from "../../lib/loop-overrides";
import { LOOP_BUDGET_POLICY_LABELS, LOOP_LIMIT_LABELS } from "../../lib/loop-limits";
import type { LoopEffectiveConfig } from "../../types";
import { LoopRailSection } from "../loop-rail-section";

interface LoopRunOverridesProps {
  effectiveConfig: LoopEffectiveConfig;
  draft: LoopOverrideDraft;
  disabled?: boolean;
  onChange: (draft: LoopOverrideDraft) => void;
}

function setOverrideValue(
  draft: LoopOverrideDraft,
  field: LoopOverrideField,
  raw: string
): LoopOverrideDraft {
  const values = { ...draft.values };
  if (raw.trim() === "") {
    delete values[field.key];
    return { ...draft, values };
  }
  const parsed = Number(raw);
  if (Number.isNaN(parsed)) return draft;
  values[field.key] = clampOverrideValue(field, parsed);
  return { ...draft, values };
}

export function LoopRunOverrides({
  effectiveConfig,
  draft,
  disabled,
  onChange,
}: LoopRunOverridesProps) {
  const fields = buildOverrideFields(effectiveConfig);
  return (
    <LoopRailSection
      data-testid="loop-run-overrides"
      gist={
        <span data-testid="loop-run-overrides-badge">
          {summarizeRunLimits(draft, effectiveConfig)}
        </span>
      }
      icon={<Gauge aria-hidden="true" className="size-3.5" />}
      title="Limits"
    >
      <div className="flex flex-col px-4 py-1">
        {fields.map(field => (
          <div
            key={field.key}
            className="flex items-center justify-between gap-2.5 border-t border-line-soft py-2 first:border-t-0"
            data-testid={`loop-run-override-${field.key}`}
          >
            <label
              className="text-form-label text-subtle"
              htmlFor={`loop-run-override-input-${field.key}`}
            >
              {field.label}
            </label>
            <div className="flex items-center gap-2">
              <Input
                id={`loop-run-override-input-${field.key}`}
                data-testid={`loop-run-override-input-${field.key}`}
                type="number"
                min={0}
                max={field.ceiling}
                className="w-24 font-mono text-form-input"
                disabled={disabled}
                placeholder={
                  field.defaultValue !== null ? String(field.defaultValue) : field.placeholder
                }
                value={draft.values[field.key] !== undefined ? String(draft.values[field.key]) : ""}
                onChange={event => onChange(setOverrideValue(draft, field, event.target.value))}
              />
              <span className="shrink-0 text-form-hint whitespace-nowrap text-faint">
                {field.ceilingLabel}
              </span>
            </div>
          </div>
        ))}
        <div
          className="flex items-center justify-between gap-2.5 border-t border-line-soft py-2"
          data-testid="loop-run-override-budget_on_exceeded"
        >
          <label className="text-form-label text-subtle" htmlFor="loop-run-override-policy">
            {LOOP_LIMIT_LABELS.budget_on_exceeded}
          </label>
          <NativeSelect
            id="loop-run-override-policy"
            data-testid="loop-run-override-policy"
            className="w-40 text-form-input"
            disabled={disabled}
            value={draft.budgetOnExceeded}
            onChange={event =>
              onChange({ ...draft, budgetOnExceeded: event.target.value as LoopBudgetPolicy })
            }
          >
            <NativeSelectOption value="halt">{LOOP_BUDGET_POLICY_LABELS.halt}</NativeSelectOption>
            <NativeSelectOption value="escalate">
              {LOOP_BUDGET_POLICY_LABELS.escalate}
            </NativeSelectOption>
          </NativeSelect>
        </div>
      </div>
      <p className="border-t border-line-soft px-4 py-3 text-form-hint text-faint">
        These limits apply to this run only.
      </p>
    </LoopRailSection>
  );
}
