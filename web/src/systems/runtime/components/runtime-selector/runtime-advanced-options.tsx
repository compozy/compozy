import { ChevronDown, SlidersHorizontal } from "lucide-react";
import { useId } from "react";

import {
  cn,
  Eyebrow,
  NativeSelect,
  NativeSelectOptGroup,
  NativeSelectOption,
  Switch,
} from "@compozy/ui";

import type { RuntimeACPOption, RuntimeACPOptionSelection } from "./types";

export interface RuntimeAdvancedOptionsProps {
  /** Controller-normalized advanced descriptors in stable ID order. */
  options: readonly RuntimeACPOption[];
  selections: readonly RuntimeACPOptionSelection[];
  expanded: boolean;
  disabled?: boolean;
  providerManaged?: boolean;
  onExpandedChange: (expanded: boolean) => void;
  onChange: (selection: RuntimeACPOptionSelection | null) => void;
}

type RuntimeAdvancedOptionControlProps = Pick<
  RuntimeAdvancedOptionsProps,
  "selections" | "disabled" | "onChange"
> & {
  option: RuntimeACPOption;
  descriptionId?: string;
};

function optionLabel(option: RuntimeACPOption): string {
  return option.label?.trim() || option.id;
}

function optionDescription(option: RuntimeACPOption): string | undefined {
  const description = option.description?.trim();
  return description && description !== optionLabel(option) ? description : undefined;
}

function selectedValue(
  option: RuntimeACPOption,
  selections: readonly RuntimeACPOptionSelection[]
): string {
  const selected = selections.find(selection => selection.id === option.id);
  const candidate = selected?.value_id ?? option.current_value_id ?? "";
  return option.values?.some(value => value.value === candidate) ? candidate : "";
}

function selectedBoolean(
  option: RuntimeACPOption,
  selections: readonly RuntimeACPOptionSelection[]
): boolean {
  const selected = selections.find(selection => selection.id === option.id);
  return selected?.bool_value ?? option.current_bool ?? false;
}

function SelectOption({
  option,
  selections,
  disabled,
  descriptionId,
  onChange,
}: RuntimeAdvancedOptionControlProps) {
  const values = option.values ?? [];
  const value = selectedValue(option, selections);
  const ungrouped = values.filter(candidate => !candidate.group_id && !candidate.group_label);
  const groups = new Map<string, typeof values>();
  for (const candidate of values) {
    const groupId = candidate.group_id?.trim() || candidate.group_label?.trim();
    if (!groupId) continue;
    const group = groups.get(groupId) ?? [];
    group.push(candidate);
    groups.set(groupId, group);
  }

  return (
    <NativeSelect
      aria-describedby={descriptionId}
      aria-label={optionLabel(option)}
      className="min-w-0 flex-1"
      data-testid={`runtime-selector-option-${option.id}`}
      disabled={disabled || values.length === 0}
      onChange={event => {
        const next = event.target.value;
        onChange(next ? { id: option.id, value_id: next } : null);
      }}
      size="sm"
      value={value}
    >
      <NativeSelectOption value="">Provider default</NativeSelectOption>
      {ungrouped.map(candidate => (
        <NativeSelectOption key={candidate.value} value={candidate.value}>
          {candidate.label?.trim() || candidate.value}
        </NativeSelectOption>
      ))}
      {[...groups.entries()].map(([groupId, group]) => (
        <NativeSelectOptGroup key={groupId} label={group[0]?.group_label?.trim() || groupId}>
          {group.map(candidate => (
            <NativeSelectOption key={candidate.value} value={candidate.value}>
              {candidate.label?.trim() || candidate.value}
            </NativeSelectOption>
          ))}
        </NativeSelectOptGroup>
      ))}
    </NativeSelect>
  );
}

function BooleanOption({
  option,
  selections,
  disabled,
  descriptionId,
  onChange,
}: RuntimeAdvancedOptionControlProps) {
  return (
    <Switch
      aria-describedby={descriptionId}
      aria-label={optionLabel(option)}
      checked={selectedBoolean(option, selections)}
      data-testid={`runtime-selector-option-${option.id}`}
      disabled={disabled}
      onCheckedChange={checked => onChange({ id: option.id, bool_value: checked })}
      size="sm"
    />
  );
}

function UnsupportedOption({ option }: { option: RuntimeACPOption }) {
  return (
    <span
      className="shrink-0 text-badge text-subtle"
      data-testid={`runtime-selector-option-${option.id}-unsupported`}
    >
      Not available
    </span>
  );
}

function OptionRow({
  option,
  selections,
  disabled,
  descriptionId,
  onChange,
}: RuntimeAdvancedOptionControlProps) {
  const label = optionLabel(option);
  const description = optionDescription(option);
  const supported = option.kind === "select" || option.kind === "boolean";
  return (
    <div
      className={cn(
        "flex min-h-9 items-center gap-3 border-t border-line-soft px-3 py-1.5",
        !supported && "opacity-60"
      )}
      data-kind={option.kind}
      data-option-id={option.id}
    >
      <span className="min-w-0 flex-1">
        <span className="block truncate text-small-body text-fg">{label}</span>
        {description ? (
          <span
            className="block truncate text-form-hint text-subtle"
            data-testid={`runtime-selector-option-${option.id}-description`}
            id={descriptionId}
          >
            {description}
          </span>
        ) : null}
      </span>
      {option.kind === "select" ? (
        <SelectOption
          descriptionId={description ? descriptionId : undefined}
          disabled={disabled}
          onChange={onChange}
          option={option}
          selections={selections}
        />
      ) : option.kind === "boolean" ? (
        <BooleanOption
          descriptionId={description ? descriptionId : undefined}
          disabled={disabled}
          onChange={onChange}
          option={option}
          selections={selections}
        />
      ) : (
        <UnsupportedOption option={option} />
      )}
    </div>
  );
}

/**
 * Progressive disclosure for ACP controls that are neither model, reasoning,
 * nor Fast. It consumes the live descriptor, keeps grouped values intact, and
 * emits typed selections so a provider never receives a stringified boolean.
 */
export function RuntimeAdvancedOptions({
  options,
  selections,
  expanded,
  disabled = false,
  providerManaged = false,
  onExpandedChange,
  onChange,
}: RuntimeAdvancedOptionsProps) {
  const instanceId = useId();
  if (options.length === 0) return null;

  const panelId = `${instanceId}-panel`;
  const controlsDisabled = disabled || providerManaged;
  return (
    <section className="shrink-0 border-t border-line-soft" data-testid="runtime-selector-advanced">
      <button
        type="button"
        aria-controls={panelId}
        aria-expanded={expanded}
        className="flex h-8 w-full items-center gap-2 px-3 text-left text-badge text-subtle outline-none transition-colors hover:bg-row-hover hover:text-fg focus-visible:bg-row-hover focus-visible:text-fg-strong focus-visible:ring-2 focus-visible:ring-accent"
        data-testid="runtime-selector-advanced-toggle"
        onClick={() => onExpandedChange(!expanded)}
      >
        <SlidersHorizontal aria-hidden="true" className="size-3.5 shrink-0" />
        <Eyebrow className="min-w-0 flex-1 text-left text-subtle">Advanced options</Eyebrow>
        {providerManaged ? (
          <span className="shrink-0 text-badge text-warning">Provider managed</span>
        ) : null}
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "size-3.5 shrink-0 transition-transform motion-reduce:transition-none",
            expanded && "rotate-180"
          )}
        />
      </button>
      {expanded ? (
        <div
          aria-disabled={controlsDisabled || undefined}
          className={cn("bg-canvas-soft", controlsDisabled && "opacity-60")}
          data-testid="runtime-selector-advanced-panel"
          id={panelId}
        >
          {options.map(option => (
            <OptionRow
              disabled={controlsDisabled || (option.kind !== "select" && option.kind !== "boolean")}
              key={option.id}
              onChange={onChange}
              option={option}
              selections={selections}
              descriptionId={`${instanceId}-${option.id}-description`}
            />
          ))}
          {providerManaged ? (
            <p className="border-t border-line-soft px-3 py-1.5 text-form-hint text-warning">
              This provider controls these settings.
            </p>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
