import { Input, NativeSelect, NativeSelectOption } from "@agh/ui";

import {
  buildOverrideFields,
  clampOverrideValue,
  type LoopBudgetPolicy,
  type LoopOverrideDraft,
  type LoopOverrideField,
} from "../../lib/loop-overrides";
import type { LoopEffectiveConfig } from "../../types";

interface LoopConfigureLimitsProps {
  effectiveConfig: LoopEffectiveConfig;
  draft: LoopOverrideDraft;
  disabled?: boolean;
  onChange: (draft: LoopOverrideDraft) => void;
}

/** Sets one numeric override, clearing it on empty and clamping to the field ceiling. */
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

/**
 * The per-loop "Stop limits" grid (§4.7.4): the same 6 clamped fields as the run-form
 * Advanced panel plus the budget-exceeded policy, each showing its per-loop default and
 * hard daemon ceiling. Values clamp at the ceiling. There is NO cost-cap field — cost is
 * display-only (ADR-017 §9.5.2).
 */
export function LoopConfigureLimits({
  effectiveConfig,
  draft,
  disabled = false,
  onChange,
}: LoopConfigureLimitsProps) {
  const fields = buildOverrideFields(effectiveConfig);
  return (
    <div data-testid="loop-configure-limits">
      <div className="grid grid-cols-1 gap-x-4 gap-y-3.5 sm:grid-cols-2">
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
                className="h-8 font-mono text-form-input"
                disabled={disabled}
                placeholder={
                  field.defaultValue !== null ? String(field.defaultValue) : field.placeholder
                }
                value={draft.values[field.key] !== undefined ? String(draft.values[field.key]) : ""}
                onChange={event => onChange(setOverrideValue(draft, field, event.target.value))}
              />
              <span className="shrink-0 font-mono text-mono-id whitespace-nowrap text-faint">
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
            Budget on exceeded
          </label>
          <NativeSelect
            id="loop-configure-limit-policy"
            data-testid="loop-configure-limit-policy"
            className="h-8 font-mono text-form-input"
            disabled={disabled}
            value={draft.budgetOnExceeded}
            onChange={event =>
              onChange({ ...draft, budgetOnExceeded: event.target.value as LoopBudgetPolicy })
            }
          >
            <NativeSelectOption value="halt">halt</NativeSelectOption>
            <NativeSelectOption value="escalate">escalate</NativeSelectOption>
          </NativeSelect>
        </div>
      </div>
      <p className="mt-3.5 border-t border-line-soft pt-3 text-form-hint leading-relaxed text-faint">
        These are this loop's saved defaults, applied to every future run and bounded by the hard
        daemon ceilings, which can't be raised here. Token and wall-clock budgets are opt-in (0 =
        unlimited); a set budget is enforced. On exceeded, halt ends the run as exhausted and
        escalate pauses it as needs-approval. Cost is a display-only estimate, never a cap.
      </p>
    </div>
  );
}
