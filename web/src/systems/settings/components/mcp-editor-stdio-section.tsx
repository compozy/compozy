import { Button, FormSection, Input } from "@agh/ui";
import { Plus, Trash2 } from "lucide-react";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type {
  MCPDraft,
  MCPDraftErrors,
  MCPEnvPair,
  MCPSecretBinding,
  MCPSecretEnvEntry,
} from "../lib/mcp-editor-model";

import { MCPFieldLabel } from "./mcp-editor-fields";
import { MCPSecretBindingControl, type MCPVaultInventory } from "./mcp-secret-binding";

export interface MCPEditorProcessSectionProps {
  draft: MCPDraft;
  errors: MCPDraftErrors;
  vaultInventory: MCPVaultInventory;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
}

/**
 * Advanced tier for a local server: the process environment.
 *
 * Only stdio carries these — `toRequest` omits command, args, env, and
 * secret_env entirely on a remote definition — so the caller renders this
 * section only while the local transport is selected.
 */
export function MCPEditorProcessSection({
  draft,
  errors,
  vaultInventory,
  onChange,
}: MCPEditorProcessSectionProps) {
  return (
    <>
      <FormSection
        data-testid="settings-mcp-editor-stdio"
        description="Ordered arguments and environment for the spawned process."
        rightLabel="stdio only"
        title="Process environment"
      >
        <div className="flex flex-col gap-3">
          <ArgsEditor
            args={draft.args}
            onChange={args => onChange(current => ({ ...current, args }))}
          />
          <EnvEditor
            env={draft.env}
            errors={errors.env}
            onChange={env => onChange(current => ({ ...current, env }))}
          />
        </div>
      </FormSection>

      <SecretEnvSection
        secretEnv={draft.secretEnv}
        errors={errors.secretEnv}
        vaultInventory={vaultInventory}
        onChange={secretEnv => onChange(current => ({ ...current, secretEnv }))}
      />
    </>
  );
}

function ArgsEditor({ args, onChange }: { args: string[]; onChange: (next: string[]) => void }) {
  const rowKeys = useLocalRowKeys(args, "mcp-arg");
  return (
    <div>
      <MCPFieldLabel hint="ordered rows">Args</MCPFieldLabel>
      <div className="flex flex-col gap-1.5" data-testid="settings-mcp-servers-editor-args-list">
        {args.map((arg, index) => (
          <div key={rowKeys.keys[index]} className="flex items-center gap-2">
            <span className="w-5 text-center font-mono text-caption text-subtle">{index + 1}</span>
            <Input
              className="flex-1 font-mono"
              aria-label={`Argument ${index + 1}`}
              value={arg}
              placeholder={`arg[${index}]`}
              data-testid={`settings-mcp-servers-editor-args-input-${index}`}
              onChange={event => {
                const next = [...args];
                next[index] = event.target.value;
                onChange(next);
              }}
            />
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={`Remove argument ${index + 1}`}
              data-testid={`settings-mcp-servers-editor-args-remove-${index}`}
              onClick={() => {
                rowKeys.remove(index);
                onChange(args.filter((_, i) => i !== index));
              }}
            >
              <Trash2 className="size-3" />
            </Button>
          </div>
        ))}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="justify-self-start"
          data-testid="settings-mcp-servers-editor-args-add"
          onClick={() => {
            rowKeys.append();
            onChange([...args, ""]);
          }}
        >
          <Plus className="size-3" />
          Add argument
        </Button>
      </div>
    </div>
  );
}

function EnvEditor({
  env,
  errors,
  onChange,
}: {
  env: MCPEnvPair[];
  errors?: Record<number, string>;
  onChange: (next: MCPEnvPair[]) => void;
}) {
  const rowKeys = useLocalRowKeys(env, "mcp-env");
  return (
    <div>
      <MCPFieldLabel hint="key / value">Non-secret environment</MCPFieldLabel>
      <div className="flex flex-col gap-1.5" data-testid="settings-mcp-servers-editor-env-list">
        {env.map((pair, index) => (
          <div key={rowKeys.keys[index]}>
            <div className="flex items-center gap-2">
              <Input
                className="w-40 font-mono"
                aria-label="Environment key"
                aria-invalid={Boolean(errors?.[index])}
                value={pair.key}
                placeholder="KEY"
                data-testid={`settings-mcp-servers-editor-env-key-${index}`}
                onChange={event => {
                  const next = [...env];
                  next[index] = { ...pair, key: event.target.value };
                  onChange(next);
                }}
              />
              <Input
                className="flex-1 font-mono"
                aria-label="Environment value"
                aria-invalid={Boolean(errors?.[index])}
                value={pair.value}
                placeholder={pair.existing && !pair.valueChanged ? "Configured value" : "value"}
                data-testid={`settings-mcp-servers-editor-env-value-${index}`}
                onChange={event => {
                  const next = [...env];
                  next[index] = { ...pair, value: event.target.value, valueChanged: true };
                  onChange(next);
                }}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label="Remove environment variable"
                data-testid={`settings-mcp-servers-editor-env-remove-${index}`}
                onClick={() => {
                  rowKeys.remove(index);
                  onChange(env.filter((_, i) => i !== index));
                }}
              >
                <Trash2 className="size-3" />
              </Button>
            </div>
            {errors?.[index] ? (
              <p className="mt-1 text-caption text-danger" role="alert">
                {errors[index]}
              </p>
            ) : null}
          </div>
        ))}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          data-testid="settings-mcp-servers-editor-env-add"
          onClick={() => {
            rowKeys.append();
            onChange([...env, { key: "", value: "" }]);
          }}
        >
          <Plus className="size-3" />
          Add variable
        </Button>
      </div>
    </div>
  );
}

function SecretEnvSection({
  secretEnv,
  errors,
  vaultInventory,
  onChange,
}: {
  secretEnv: MCPSecretEnvEntry[];
  errors?: Record<number, string>;
  vaultInventory: MCPVaultInventory;
  onChange: (next: MCPSecretEnvEntry[]) => void;
}) {
  const rowKeys = useLocalRowKeys(secretEnv, "mcp-secret");
  const updateEntry = (index: number, patch: Partial<MCPSecretEnvEntry>) => {
    onChange(secretEnv.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)));
  };
  return (
    <FormSection
      title="Secret environment"
      rightLabel="hybrid binding"
      data-testid="settings-mcp-editor-stdio-secret"
    >
      <div className="flex flex-col gap-4">
        {secretEnv.map((entry, index) => {
          const rowError = errors?.[index];
          return (
            <div
              key={rowKeys.keys[index]}
              className="flex flex-col gap-2 border-t border-line-soft pt-3 first:border-t-0 first:pt-0"
            >
              <div className="flex items-center gap-2">
                <Input
                  className="flex-1 font-mono"
                  aria-label="Secret environment key"
                  aria-invalid={rowError ? true : undefined}
                  value={entry.key}
                  placeholder="GITHUB_TOKEN"
                  data-testid={`settings-mcp-servers-editor-secret-key-${index}`}
                  onChange={event => updateEntry(index, { key: event.target.value })}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label="Remove secret binding"
                  data-testid={`settings-mcp-servers-editor-secret-remove-${index}`}
                  onClick={() => {
                    rowKeys.remove(index);
                    onChange(secretEnv.filter((_, i) => i !== index));
                  }}
                >
                  <Trash2 className="size-3" />
                </Button>
              </div>
              <MCPSecretBindingControl
                binding={entry.binding}
                vaultInventory={vaultInventory}
                idPrefix={`mcp-secret-${index}`}
                onChange={binding => updateEntry(index, { binding })}
              />
              {rowError ? (
                <p
                  className="text-caption text-danger"
                  data-testid={`settings-mcp-servers-editor-secret-error-${index}`}
                >
                  {rowError}
                </p>
              ) : null}
            </div>
          );
        })}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          data-testid="settings-mcp-servers-editor-secret-add"
          onClick={() => {
            rowKeys.append();
            onChange([...secretEnv, { key: "", binding: emptyBinding() }]);
          }}
        >
          <Plus className="size-3" />
          Add secret binding
        </Button>
        <p className="rounded-md bg-info-tint p-2.5 text-caption text-info">
          <strong className="font-semibold">Bindings stay hidden.</strong> Configured bindings can
          be kept by name. Typed replacements are write-only, and Vault replacements come only from
          the explicit inventory.
        </p>
      </div>
    </FormSection>
  );
}

function emptyBinding(): MCPSecretBinding {
  return { mode: "typed", existing: false, typedValue: "", vaultRef: "" };
}
