import { Plus } from "lucide-react";
import { useId } from "react";

import { Button, FieldDescription } from "@compozy/ui";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import { automationCondition, type AutomationCondition } from "../../../lib/automation-form-draft";
import { conditionFieldLabel, conditionFieldOptions } from "../../../lib/automation-form-events";
import { AutomationFormSection } from "../automation-form-section";
import { ConditionRow } from "./condition-row";

interface OnlyIfSectionProps {
  conditions: readonly AutomationCondition[];
  form: AutomationFormModel;
}

/** 03 Only if (events and links): optional exact-match conditions, all of which must match. */
export function OnlyIfSection({ conditions, form }: OnlyIfSectionProps) {
  const datalistId = useId();
  const fieldOptions = conditionFieldOptions(form.eventDef);
  const openPayload = form.eventDef ? form.eventDef.openPayload === true : true;

  const update = (id: string, patch: Partial<AutomationCondition>) =>
    form.onConditionsChange(
      conditions.map(condition => (condition.id === id ? { ...condition, ...patch } : condition))
    );
  const handleAdd = () => {
    const used = new Set(conditions.map(condition => condition.key));
    // Prefer a payload field: envelope keys rarely narrow an event.
    const candidates = [
      ...fieldOptions.filter(option => option.startsWith("data.")),
      ...fieldOptions,
    ];
    const nextKey = candidates.find(option => !used.has(option)) ?? "";
    form.onConditionsChange([...conditions, automationCondition(nextKey)]);
  };

  return (
    <AutomationFormSection
      data-testid="automation-form-only-if"
      description="Optional. Every condition must match."
      number={3}
      title="Only if"
    >
      {openPayload ? (
        <datalist id={datalistId}>
          {fieldOptions.map(option => (
            <option key={option} value={option}>
              {conditionFieldLabel(option)}
            </option>
          ))}
        </datalist>
      ) : null}
      {conditions.length > 0 ? (
        <div className="flex flex-col gap-2">
          {conditions.map((condition, index) => (
            <ConditionRow
              datalistId={datalistId}
              field={condition.key}
              fieldOptions={fieldOptions}
              index={index}
              key={condition.id}
              onFieldChange={key => update(condition.id, { key })}
              onRemove={() =>
                form.onConditionsChange(conditions.filter(row => row.id !== condition.id))
              }
              onValueChange={value => update(condition.id, { value })}
              openPayload={openPayload}
              problem={form.conditionProblems.get(index)}
              value={condition.value}
            />
          ))}
        </div>
      ) : null}
      <div className="flex flex-col items-start gap-1.5">
        <Button
          data-testid="automation-condition-add"
          onClick={handleAdd}
          size="xs"
          type="button"
          variant="ghost"
        >
          <Plus aria-hidden="true" />
          Add condition
        </Button>
        <FieldDescription>With no conditions, every event of this kind starts it.</FieldDescription>
      </div>
    </AutomationFormSection>
  );
}
