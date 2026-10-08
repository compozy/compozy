import { Plus } from "lucide-react";
import { useId } from "react";

import { Button, FieldDescription } from "@compozy/ui";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import { conditionFieldLabel, conditionFieldOptions } from "../../../lib/automation-form-events";
import type { AutomationTriggerFilter } from "../../../types";
import { AutomationFormSection } from "../automation-form-section";
import { ConditionRow } from "./condition-row";

type Entry = [string, string];

interface OnlyIfSectionProps {
  filter: AutomationTriggerFilter;
  form: AutomationFormModel;
}

/** 03 Only if (events and links): optional exact-match conditions, all of which must match. */
export function OnlyIfSection({ filter, form }: OnlyIfSectionProps) {
  const datalistId = useId();
  const rows = Object.entries(filter) as Entry[];
  const fieldOptions = conditionFieldOptions(form.eventDef);
  const openPayload = form.eventDef ? form.eventDef.openPayload === true : true;

  const commit = (next: Entry[]) => form.onFilterChange(Object.fromEntries(next));
  const handleAdd = () => {
    const used = new Set(rows.map(([key]) => key));
    // Prefer a payload field: envelope keys rarely narrow an event.
    const candidates = [
      ...fieldOptions.filter(option => option.startsWith("data.")),
      ...fieldOptions,
    ];
    const nextKey = candidates.find(option => !used.has(option)) ?? "";
    commit([...rows, [nextKey, ""]]);
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
      {rows.length > 0 ? (
        <div className="flex flex-col gap-2">
          {rows.map(([field, value], index) => (
            <ConditionRow
              datalistId={datalistId}
              field={field}
              fieldOptions={fieldOptions}
              incomplete={form.incompleteConditions.has(index)}
              index={index}
              key={field || "new-condition"}
              onFieldChange={next =>
                commit(rows.map((row, i) => (i === index ? [next, row[1]] : row)))
              }
              onRemove={() => commit(rows.filter((_, i) => i !== index))}
              onValueChange={next =>
                commit(rows.map((row, i) => (i === index ? [row[0], next] : row)))
              }
              openPayload={openPayload}
              value={value}
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
