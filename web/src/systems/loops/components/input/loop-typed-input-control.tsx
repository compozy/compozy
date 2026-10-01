import { Plus, X } from "lucide-react";

import { Button, Input, Switch, cn } from "@compozy/ui";

import { AgentCommandSelect } from "@/systems/agent";
import { WorktreeRefSelect } from "@/systems/workspace";
import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type { LoopInputSchemaField } from "../../types";
import type { LoopEntityKind } from "../../lib/loop-input-kinds";
import type { LoopEntityCatalog } from "../../lib/loop-input-catalogs";
import { useLoopInputCatalogs } from "../../hooks/use-loop-input-catalogs";
import { LoopSessionValueSelect } from "./loop-session-value-select";
import { LoopCatalogValueSelect } from "./loop-catalog-value-select";
import { LoopRuntimeValueControl } from "./loop-runtime-value-control";

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
  if (kind === "session") {
    return (
      <LoopSessionValueSelect
        workspaceId={catalogs.sessionWorkspaceId ?? ""}
        value={value}
        controlId={controlId}
        testId={testId}
        disabled={disabled}
        invalid={invalid}
        describedBy={describedBy}
        onChange={onChange}
      />
    );
  }
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
        variant="secondary"
      >
        <Plus aria-hidden="true" />
        Add {kind}
      </Button>
    </div>
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

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function enumCatalog(options: readonly string[]): LoopEntityCatalog {
  return {
    options: options.map(option => ({ value: option, label: option })),
    loading: false,
    error: null,
  };
}

/** Entity kind a field picks from a catalog, or null for plain value fields. */
function fieldEntityKind(field: LoopInputSchemaField): LoopEntityKind | null {
  if (field.type === "agent") return "agent";
  if (field.type === "ref" && field.ref?.kind) return field.ref.kind;
  return null;
}

function scalarPlaceholder(defaultValue: unknown): string | undefined {
  if (defaultValue === undefined || defaultValue === null || typeof defaultValue === "object") {
    return undefined;
  }
  return String(defaultValue) || undefined;
}

function scalarInputValue(value: unknown, isNumber: boolean): string {
  if (isNumber) return typeof value === "number" ? String(value) : "";
  return stringValue(value);
}

type LoopScalarInputProps = Omit<LoopTypedInputControlProps, "field"> & {
  isNumber: boolean;
  placeholder?: string;
};

function LoopScalarInput({
  isNumber,
  placeholder,
  value,
  controlId,
  testId,
  disabled,
  invalid,
  describedBy,
  onChange,
}: LoopScalarInputProps) {
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
      value={scalarInputValue(value, isNumber)}
    />
  );
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
  const shared = { controlId, testId, disabled, invalid, describedBy };
  if (field.type === "string" && field.enum && field.enum.length > 0) {
    return (
      <LoopCatalogValueSelect
        {...shared}
        allowManual={false}
        catalog={enumCatalog(field.enum)}
        label="value"
        onChange={next => onChange(next)}
        value={stringValue(value)}
      />
    );
  }
  const entityKind = fieldEntityKind(field);
  if (entityKind !== null) {
    return (
      <LoopEntityValueControl
        {...shared}
        kind={entityKind}
        onChange={next => onChange(next)}
        value={stringValue(value)}
      />
    );
  }
  if (field.type === "runtime") {
    return <LoopRuntimeValueControl {...shared} onChange={onChange} value={value} />;
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
  return (
    <LoopScalarInput
      {...shared}
      isNumber={field.type === "number"}
      onChange={onChange}
      placeholder={scalarPlaceholder(field.default)}
      value={value}
    />
  );
}
