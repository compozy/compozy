import { Input, NativeSelect, NativeSelectOption } from "@compozy/ui";

import {
  buildOverrideFields,
  clampOverrideValue,
  type LoopBudgetPolicy,
  type LoopOverrideDraft,
  type LoopOverrideField,
} from "../../lib/loop-overrides";
import { LOOP_BUDGET_POLICY_LABELS, LOOP_LIMIT_LABELS } from "../../lib/loop-limits";
import type { LoopEffectiveConfig } from "../../types";

interface LoopConfigureLimitsProps {
  effectiveConfig: LoopEffectiveConfig;
  draft: LoopOverrideDraft;
  disabled?: boolean;
  onChange: (draft: LoopOverrideDraft) => void;
}

/** Sets one numeric override, clearing it on empty and applying any field ceiling. */
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

export function LoopConfigureLimits({
  effectiveConfig,
  draft,
  disabled = false,
  onChange,
}: LoopConfigureLimitsProps) {
  const fields = buildOverrideFields(effectiveConfig);
  return (
    <div data-testid="loop-configure-limits">
      <div className="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
        {fields.map(field => (
          <div
            key={field.key}
            className="flex flex-col gap-1.5"
            data-testid={`loop-configure-limit-${field.key}`}
          >
            <label
              className="text-form-label font-medium text-fg-strong"
              htmlFor={`loop-configure-limit-input-${field.key}`}
            >
              {field.label}
            </label>
            <div className="flex items-center gap-2">
              <Input
                id={`loop-configure-limit-input-${field.key}`}
                data-testid={`loop-configure-limit-input-${field.key}`}
                type="number"
                min={0}
                max={field.ceiling}
                className="font-mono text-form-input"
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
          className="flex flex-col gap-1.5"
          data-testid="loop-configure-limit-budget_on_exceeded"
        >
          <label
            className="text-form-label font-medium text-fg-strong"
            htmlFor="loop-configure-limit-policy"
          >
            {LOOP_LIMIT_LABELS.budget_on_exceeded}
          </label>
          <NativeSelect
            id="loop-configure-limit-policy"
            data-testid="loop-configure-limit-policy"
            className="text-form-input"
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
      <p className="mt-3 border-t border-line-soft pt-3 text-form-hint text-faint">
        Saved as the default for future runs.
      </p>
    </div>
  );
}
