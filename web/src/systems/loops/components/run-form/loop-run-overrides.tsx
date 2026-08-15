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

/**
 * Per-run limits (§4.3): folded by default but never silent — the summary states the
 * generations, whether a budget is enforced, and whether the loop's saved defaults are
 * still in play, so folding hides detail rather than truth. Open, it is the limrow list
 * of 6 clamped fields (each showing its per-loop default + daemon ceiling) plus the
 * budget-exceeded policy as the last row. There is NO cost-cap field — cost is
 * display-only (ADR-017 §9.5.2).
 */
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
      <div className="flex flex-col px-3.5 py-1">
        {fields.map(field => (
          <div
            key={field.key}
            className="flex items-center justify-between gap-2.5 border-t border-line-soft py-2 first:border-t-0"
            data-testid={`loop-run-override-${field.key}`}
          >
            <label className="text-xs text-subtle" htmlFor={`loop-run-override-input-${field.key}`}>
              {field.label}
            </label>
            <div className="flex items-center gap-2">
              <Input
                id={`loop-run-override-input-${field.key}`}
                data-testid={`loop-run-override-input-${field.key}`}
                type="number"
                min={0}
                max={field.ceiling}
                className="h-8 w-24 font-mono text-form-input"
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
          className="flex items-center justify-between gap-2.5 border-t border-line-soft py-2"
          data-testid="loop-run-override-budget_on_exceeded"
        >
          <label className="text-xs text-subtle" htmlFor="loop-run-override-policy">
            Budget on exceeded
          </label>
          <NativeSelect
            id="loop-run-override-policy"
            data-testid="loop-run-override-policy"
            className="h-8 w-32 font-mono text-form-input"
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
      <p className="border-t border-line-soft px-3.5 py-3 text-form-hint leading-relaxed text-faint">
        Overrides apply to this run only and never change the loop's saved defaults. Structural
        ceilings are hard backstops that cannot be raised; a set token or wall-clock budget is
        enforced (0 = unlimited). On exceeded, halt ends the run as exhausted and escalate pauses it
        as needs-approval. Cost is a display-only estimate, never a cap.
      </p>
    </LoopRailSection>
  );
}
