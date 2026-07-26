import { useId } from "react";
import { Plus } from "lucide-react";

import { Button } from "@agh/ui";

import type { AutomationTriggerFilter } from "../../types";
import { FilterRow } from "./filter-row";

interface FilterConditionsProps {
  filter: AutomationTriggerFilter;
  eventKind: string;
  keyOptions: string[];
  openPayload: boolean;
  onChange: (next: AutomationTriggerFilter) => void;
}

type Entry = [string, string];

/** "Only if" step: structured AND conditions over the event envelope. */
export function FilterConditions({
  filter,
  eventKind,
  keyOptions,
  openPayload,
  onChange,
}: FilterConditionsProps) {
  const datalistId = useId();
  const rows = Object.entries(filter) as Entry[];

  const commit = (next: Entry[]) => onChange(Object.fromEntries(next));

  const handleAdd = () => {
    const used = new Set(rows.map(([key]) => key));
    const nextKey = keyOptions.find(option => !used.has(option)) ?? keyOptions[0] ?? "";
    commit([...rows, [nextKey, ""]]);
  };

  if (rows.length === 0) {
    return (
      <div>
        <div className="mb-2 rounded-md border border-dashed border-line-soft bg-canvas-tint px-3 py-2.5 text-form-label text-subtle">
          No conditions, fires on every{" "}
          <b className="font-mono text-form-hint font-medium text-fg">{eventKind}</b> event.
        </div>
        <Button onClick={handleAdd} size="xs" type="button" variant="neutral">
          <Plus aria-hidden="true" className="size-3" />
          Add condition
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {openPayload ? (
        <datalist id={datalistId}>
          {keyOptions.map(option => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </datalist>
      ) : null}
      {rows.map(([conditionKey, value], index) => (
        <FilterRow
          conditionKey={conditionKey}
          datalistId={datalistId}
          index={index}
          key={conditionKey || "empty-condition"}
          keyOptions={keyOptions}
          onKeyChange={next => commit(rows.map((row, i) => (i === index ? [next, row[1]] : row)))}
          onRemove={() => commit(rows.filter((_, i) => i !== index))}
          onValueChange={next => commit(rows.map((row, i) => (i === index ? [row[0], next] : row)))}
          openPayload={openPayload}
          value={value}
        />
      ))}
      {openPayload ? (
        <p className="text-form-hint leading-snug text-subtle">
          <span className="font-mono text-fg">{eventKind}</span> payloads are open; type any{" "}
          <code className="rounded-xs bg-badge-fill px-1 font-mono text-mono-id text-fg">
            data.&lt;path&gt;
          </code>{" "}
          to match a custom field.
        </p>
      ) : null}
      <div className="flex items-center">
        <Button onClick={handleAdd} size="xs" type="button" variant="neutral">
          <Plus aria-hidden="true" className="size-3" />
          Add condition
        </Button>
        <span className="ml-2.5 text-form-hint text-faint">all conditions must match (AND)</span>
      </div>
    </div>
  );
}
