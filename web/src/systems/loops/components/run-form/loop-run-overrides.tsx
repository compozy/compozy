import { ChevronRight } from "lucide-react";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Input,
  NativeSelect,
  NativeSelectOption,
} from "@agh/ui";

import {
  buildOverrideFields,
  clampOverrideValue,
  hasActiveOverrides,
  type LoopBudgetPolicy,
  type LoopOverrideDraft,
  type LoopOverrideField,
} from "../../lib/loop-overrides";
import type { LoopEffectiveConfig } from "../../types";

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
 * The Advanced per-run limit overrides (§4.3): a collapsible grid of the 6 clamped
 * fields (each showing its per-loop default + daemon ceiling) plus the budget-exceeded
 * policy. Values clamp at the ceiling and the badge flips "using loop defaults" ->
 * "overrides set" the moment any field diverges from its default. There is NO cost-cap
 * field — cost is display-only (ADR-017 §9.5.2).
 */
export function LoopRunOverrides({
  effectiveConfig,
  draft,
  disabled,
  onChange,
}: LoopRunOverridesProps) {
  const fields = buildOverrideFields(effectiveConfig);
  const overridden = hasActiveOverrides(draft, effectiveConfig);
  return (
    <Collapsible
      className="rounded-md border border-line-soft bg-canvas-tint"
      data-testid="loop-run-overrides"
    >
      <CollapsibleTrigger className="group/collapsible-trigger flex w-full items-center gap-2.5 px-3.5 py-3 text-left text-xs font-semibold text-fg-strong">
        <ChevronRight className="size-3.5 text-muted transition-transform group-data-panel-open/collapsible-trigger:rotate-90" />
        <span className="flex-1">Advanced · per-run limit overrides</span>
        <span
          className="text-form-hint font-normal text-subtle"
          data-testid="loop-run-overrides-badge"
        >
          {overridden ? "overrides set" : "using loop defaults"}
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent className="border-t border-line-soft px-3.5 pt-3 pb-4">
        <div className="grid grid-cols-1 gap-x-4 gap-y-3.5 sm:grid-cols-2">
          {fields.map(field => (
            <div
              key={field.key}
              className="flex flex-col gap-1.5"
              data-testid={`loop-run-override-${field.key}`}
            >
              <label
                className="text-form-label font-medium text-fg-strong"
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
                  className="h-8 font-mono text-form-input"
                  disabled={disabled}
                  placeholder={
                    field.defaultValue !== null ? String(field.defaultValue) : field.placeholder
                  }
                  value={
                    draft.values[field.key] !== undefined ? String(draft.values[field.key]) : ""
                  }
                  onChange={event => onChange(setOverrideValue(draft, field, event.target.value))}
                />
                <span className="shrink-0 font-mono text-mono-id whitespace-nowrap text-faint">
                  {field.ceilingLabel}
                </span>
              </div>
            </div>
          ))}
          <div className="flex flex-col gap-1.5" data-testid="loop-run-override-budget_on_exceeded">
            <label
              className="text-form-label font-medium text-fg-strong"
              htmlFor="loop-run-override-policy"
            >
              Budget on exceeded
            </label>
            <NativeSelect
              id="loop-run-override-policy"
              data-testid="loop-run-override-policy"
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
          Overrides apply to this run only and never change the loop's saved defaults. Structural
          ceilings are hard backstops that cannot be raised; a set token or wall-clock budget is
          enforced (0 = unlimited). On exceeded, halt ends the run as exhausted and escalate pauses
          it as needs-approval. Cost is a display-only estimate, never a cap.
        </p>
      </CollapsibleContent>
    </Collapsible>
  );
}
