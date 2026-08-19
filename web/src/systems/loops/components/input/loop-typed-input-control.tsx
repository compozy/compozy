import { useState } from "react";
import { Plus, X } from "lucide-react";

import {
  Button,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  Eyebrow,
  Input,
  Switch,
  cn,
} from "@compozy/ui";

import { AgentCommandSelect } from "@/systems/agent";
import {
  REASONING_EFFORT_ORDER,
  RuntimeSelector,
  type RuntimeSelectorValue,
} from "@/systems/runtime";
import { WorktreeRefSelect } from "@/systems/workspace";
import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type { LoopInputSchemaField } from "../../types";
import type { LoopEntityKind } from "../../lib/loop-input-kinds";
import { useLoopInputCatalogs } from "../../hooks/use-loop-input-catalogs";
import type { LoopEntityCatalog } from "../../lib/loop-input-catalogs";

interface EntityValueControlProps {
  kind: LoopEntityKind;
  value: string;
  controlId: string;
  testId: string;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
  onChange: (value: string) => void;
}

interface CatalogSelectProps extends Omit<EntityValueControlProps, "kind"> {
  catalog: LoopEntityCatalog;
  label: string;
  allowManual?: boolean;
}

export function LoopCatalogValueSelect({
  catalog,
  label,
  value,
  controlId,
  testId,
  disabled,
  invalid,
  describedBy,
  allowManual = true,
  onChange,
}: CatalogSelectProps) {
  const [open, setOpen] = useState(false);
  const selected = catalog.options.find(option => option.value === value);
  const missing = value !== "" && selected === undefined;

  if (catalog.error && allowManual) {
    return (
      <div className="flex flex-col gap-1.5">
        <Input
          aria-describedby={describedBy}
          aria-invalid={invalid || missing || undefined}
          className="font-mono"
          data-testid={testId}
          disabled={disabled}
          id={controlId}
          onChange={event => onChange(event.target.value)}
          placeholder={`Enter exact ${label}`}
          type="text"
          value={value}
        />
        <p className="text-form-hint text-warning" role="status">
          {catalog.error} Enter the exact value.
        </p>
      </div>
    );
  }

  return (
    <CommandSelect open={open} onOpenChange={setOpen}>
      <CommandSelectTrigger
        aria-busy={catalog.loading || undefined}
        aria-describedby={describedBy}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-invalid={invalid || missing || undefined}
        className="w-full justify-between"
        data-testid={testId}
        disabled={disabled || (catalog.loading && catalog.options.length === 0)}
        id={controlId}
        selected={Boolean(selected) || missing}
      >
        <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
          <span className="min-w-0 flex-1 truncate font-mono text-small-body text-fg">
            {(selected?.label ?? value) ||
              (catalog.loading ? `Loading ${label}s…` : `Select ${label}`)}
          </span>
          {missing ? <Eyebrow className="shrink-0 text-warning">Not available</Eyebrow> : null}
        </span>
      </CommandSelectTrigger>
      <CommandSelectShell className="min-w-64" inputPlaceholder={`Search ${label}s...`}>
        <CommandList>
          <CommandEmpty>
            {catalog.loading ? `Loading ${label}s…` : `No ${label}s match your search.`}
          </CommandEmpty>
          <CommandSelectGroup>
            {catalog.options.map(option => (
              <CommandItem
                data-checked={option.value === value ? "true" : "false"}
                key={option.value}
                onSelect={() => {
                  onChange(option.value);
                  setOpen(false);
                }}
                value={`${option.label} ${option.detail ?? ""}`}
              >
                <span className="min-w-0 flex-1 truncate text-small-body text-fg">
                  {option.label}
                </span>
                {option.detail ? (
                  <span className="truncate font-mono text-mono-id text-subtle">
                    {option.detail}
                  </span>
                ) : null}
              </CommandItem>
            ))}
          </CommandSelectGroup>
        </CommandList>
      </CommandSelectShell>
    </CommandSelect>
  );
}

export function LoopEntityValueControl({
  kind,
  value,
  controlId,
  testId,
  disabled,
  invalid,
  describedBy,
  onChange,
}: EntityValueControlProps) {
  const catalogs = useLoopInputCatalogs();
  if (kind === "agent") {
    if (catalogs.agentError) {
      return (
        <LoopCatalogValueSelect
          catalog={{ options: [], loading: false, error: catalogs.agentError }}
          controlId={controlId}
          describedBy={describedBy}
          disabled={disabled}
          invalid={invalid}
          label="agent"
          onChange={onChange}
          testId={testId}
          value={value}
        />
      );
    }
    return (
      <AgentCommandSelect
        agents={[...catalogs.agents]}
        className="w-full"
        disabled={disabled}
        error={catalogs.agentError}
        loading={catalogs.agentLoading}
        onChange={next => onChange(next ?? "")}
        triggerId={controlId}
        triggerTestId={testId}
        value={value || null}
      />
    );
  }
  if (kind === "worktree" && !catalogs.entities.worktree.error) {
    return (
      <WorktreeRefSelect
        ariaLabel="Select a worktree"
        className="w-full"
        describedBy={describedBy}
        disabled={disabled}
        invalid={invalid}
        onChange={onChange}
        testId={testId}
        triggerId={controlId}
        value={value}
        worktrees={catalogs.worktrees}
      />
    );
  }
  return (
    <LoopCatalogValueSelect
      catalog={catalogs.entities[kind]}
      controlId={controlId}
      describedBy={describedBy}
      disabled={disabled}
      invalid={invalid}
      label={kind === "secret" ? "secret reference" : kind}
      onChange={onChange}
      testId={testId}
      value={value}
    />
  );
}

function entityListValues(value: string): string[] {
  try {
    const parsed: unknown = JSON.parse(value);
    return Array.isArray(parsed) && parsed.every(entry => typeof entry === "string") ? parsed : [];
  } catch {
    return [];
  }
}

export function LoopEntityListValueControl({
  kind,
  value,
  controlId,
  testId,
  disabled,
  invalid,
  describedBy,
  onChange,
}: EntityValueControlProps) {
  const entries = entityListValues(value);
  const shownEntries = entries.length > 0 ? entries : [""];
  const rowKeys = useLocalRowKeys(shownEntries, kind);
  const emit = (next: string[]) => onChange(JSON.stringify(next));

  return (
    <div className="flex flex-col gap-2" data-testid={testId}>
      {shownEntries.map((entry, index) => {
        const entryId = index === 0 ? controlId : `${controlId}-${index}`;
        const rowKey = rowKeys.keys[index];
        return (
          <div className="flex items-center gap-2" key={rowKey}>
            <div className="min-w-0 flex-1">
              <LoopEntityValueControl
                controlId={entryId}
                describedBy={describedBy}
                disabled={disabled}
                invalid={invalid}
                kind={kind}
                onChange={next => {
                  const updated = [...shownEntries];
                  updated[index] = next;
                  emit(updated);
                }}
                testId={`${testId}-${index}`}
                value={entry}
              />
            </div>
            <Button
              aria-label={`Remove ${kind} ${index + 1}`}
              disabled={disabled || shownEntries.length === 1}
              onClick={() => {
                rowKeys.remove(index);
                emit(shownEntries.filter((_, position) => position !== index));
              }}
              size="icon-sm"
              type="button"
              variant="ghost"
            >
              <X aria-hidden="true" />
            </Button>
          </div>
        );
      })}
      <Button
        className="self-start"
        disabled={disabled}
        onClick={() => {
          rowKeys.append();
          emit([...shownEntries, ""]);
        }}
        size="sm"
        type="button"
        variant="outline"
      >
        <Plus aria-hidden="true" />
        Add {kind}
      </Button>
    </div>
  );
}

function runtimeValue(value: unknown): RuntimeSelectorValue {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return { provider: "", model: "", reasoning_effort: "" };
  }
  const record = value as Record<string, unknown>;
  const reasoning = typeof record.reasoning === "string" ? record.reasoning : "";
  const knownReasoning = new Set<string>(REASONING_EFFORT_ORDER);
  return {
    provider: typeof record.provider === "string" ? record.provider : "",
    model: typeof record.model === "string" ? record.model : "",
    reasoning_effort: knownReasoning.has(reasoning)
      ? (reasoning as RuntimeSelectorValue["reasoning_effort"])
      : "",
  };
}

function emittedRuntime(value: RuntimeSelectorValue): Record<string, string> {
  const next: Record<string, string> = {};
  if (value.provider) next.provider = value.provider;
  if (value.model) next.model = value.model;
  if (value.reasoning_effort) next.reasoning = value.reasoning_effort;
  return next;
}

function RuntimeValueControl({
  value,
  controlId,
  testId,
  disabled,
  invalid,
  onChange,
}: Omit<EntityValueControlProps, "kind" | "value" | "onChange"> & {
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const catalogs = useLoopInputCatalogs();
  const current = runtimeValue(value);
  return (
    <RuntimeSelector
      allowCustomProvider
      ariaLabelledby={undefined}
      catalogStatus={catalogs.runtimeError}
      className={cn("w-full", invalid && "border-danger")}
      disabled={disabled}
      loading={catalogs.runtimeLoading}
      models={catalogs.runtimeModels}
      onChange={next => onChange(emittedRuntime(next))}
      onRefreshCatalog={catalogs.refreshRuntime}
      providers={catalogs.runtimeProviders}
      refreshing={catalogs.refreshingRuntime}
      triggerId={controlId}
      triggerTestId={testId}
      value={current}
    />
  );
}

export interface LoopTypedInputControlProps {
  field: LoopInputSchemaField;
  value: unknown;
  controlId: string;
  testId: string;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
  onChange: (value: unknown) => void;
}

export function LoopTypedInputControl({
  field,
  value,
  controlId,
  testId,
  disabled,
  invalid,
  describedBy,
  onChange,
}: LoopTypedInputControlProps) {
  if (field.type === "string" && field.enum && field.enum.length > 0) {
    return (
      <LoopCatalogValueSelect
        allowManual={false}
        catalog={{
          options: field.enum.map(option => ({ value: option, label: option })),
          loading: false,
          error: null,
        }}
        controlId={controlId}
        describedBy={describedBy}
        disabled={disabled}
        invalid={invalid}
        label="value"
        onChange={next => onChange(next)}
        testId={testId}
        value={typeof value === "string" ? value : ""}
      />
    );
  }
  if (field.type === "agent") {
    return (
      <LoopEntityValueControl
        controlId={controlId}
        describedBy={describedBy}
        disabled={disabled}
        invalid={invalid}
        kind="agent"
        onChange={next => onChange(next)}
        testId={testId}
        value={typeof value === "string" ? value : ""}
      />
    );
  }
  if (field.type === "ref" && field.ref?.kind) {
    return (
      <LoopEntityValueControl
        controlId={controlId}
        describedBy={describedBy}
        disabled={disabled}
        invalid={invalid}
        kind={field.ref.kind}
        onChange={next => onChange(next)}
        testId={testId}
        value={typeof value === "string" ? value : ""}
      />
    );
  }
  if (field.type === "runtime") {
    return (
      <RuntimeValueControl
        controlId={controlId}
        describedBy={describedBy}
        disabled={disabled}
        invalid={invalid}
        onChange={onChange}
        testId={testId}
        value={value}
      />
    );
  }
  if (field.type === "boolean") {
    return (
      <Switch
        checked={typeof value === "boolean" ? value : Boolean(field.default)}
        data-testid={testId}
        disabled={disabled}
        id={controlId}
        onCheckedChange={checked => onChange(checked)}
      />
    );
  }
  const isNumber = field.type === "number";
  const placeholder =
    field.default === undefined || field.default === null || typeof field.default === "object"
      ? undefined
      : String(field.default) || undefined;
  return (
    <Input
      aria-describedby={describedBy}
      aria-invalid={invalid || undefined}
      className={cn("font-mono", invalid && "border-danger")}
      data-testid={testId}
      disabled={disabled}
      id={controlId}
      onChange={event => {
        if (!isNumber) {
          onChange(event.target.value);
          return;
        }
        if (event.target.value === "") {
          onChange(undefined);
          return;
        }
        const parsed = Number(event.target.value);
        if (!Number.isNaN(parsed)) onChange(parsed);
      }}
      placeholder={placeholder}
      type={isNumber ? "number" : "text"}
      value={
        isNumber
          ? typeof value === "number"
            ? String(value)
            : ""
          : typeof value === "string"
            ? value
            : ""
      }
    />
  );
}
