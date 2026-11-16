import { Button, Input } from "@compozy/ui";
import { Plus, Trash2 } from "@compozy/ui/icons";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type {
  MCPDraft,
  MCPDraftErrors,
  MCPEnvPair,
  MCPSecretBinding,
  MCPSecretEnvEntry,
} from "../lib/mcp-editor-model";

import { MCPFieldLabel } from "./mcp-editor-fields";

export interface MCPEditorProcessSectionProps {
  draft: MCPDraft;
  errors: MCPDraftErrors;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
}

/**
 * More options for a local program: ordered arguments, settings it reads, and the names of the
 * keys it needs (their values live in "Key it needs").
 *
 * Only stdio carries these — `toRequest` omits command, args, env, and secret_env entirely on a
 * remote definition — so the caller renders this only while the local transport is selected.
 */
export function MCPEditorProcessSection({ draft, errors, onChange }: MCPEditorProcessSectionProps) {
  return (
    <div className="flex flex-col gap-5" data-testid="settings-mcp-editor-stdio">
      <ArgsEditor
        args={draft.args}
        onChange={args => onChange(current => ({ ...current, args }))}
      />
      <EnvEditor
        env={draft.env}
        errors={errors.env}
        onChange={env => onChange(current => ({ ...current, env }))}
      />
      <SecretKeyNames
        secretEnv={draft.secretEnv}
        errors={errors.secretEnv}
        onChange={secretEnv => onChange(current => ({ ...current, secretEnv }))}
      />
    </div>
  );
}

function ArgsEditor({ args, onChange }: { args: string[]; onChange: (next: string[]) => void }) {
  const rowKeys = useLocalRowKeys(args, "mcp-arg");
  return (
    <div>
      <MCPFieldLabel hint="in order">Arguments</MCPFieldLabel>
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
              <Trash2 />
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
          <Plus />
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
      <MCPFieldLabel hint="not secret">Other settings it reads</MCPFieldLabel>
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
                <Trash2 />
              </Button>
            </div>
            {errors?.[index] ? (
              <p className="mt-1 text-caption text-destructive" role="alert">
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
          <Plus />
          Add setting
        </Button>
      </div>
    </div>
  );
}

function SecretKeyNames({
  secretEnv,
  errors,
  onChange,
}: {
  secretEnv: MCPSecretEnvEntry[];
  errors?: Record<number, string>;
  onChange: (next: MCPSecretEnvEntry[]) => void;
}) {
  const rowKeys = useLocalRowKeys(secretEnv, "mcp-secret");
  return (
    <div>
      <MCPFieldLabel hint="values stay hidden">Keys it needs</MCPFieldLabel>
      <div className="flex flex-col gap-1.5" data-testid="settings-mcp-servers-editor-secret-list">
        {secretEnv.map((entry, index) => (
          <div key={rowKeys.keys[index]} className="flex items-center gap-2">
            <Input
              className="flex-1 font-mono"
              aria-label="Key name"
              aria-invalid={errors?.[index] ? true : undefined}
              value={entry.key}
              placeholder="GITHUB_TOKEN"
              data-testid={`settings-mcp-servers-editor-secret-key-${index}`}
              onChange={event =>
                onChange(
                  secretEnv.map((candidate, i) =>
                    i === index ? { ...candidate, key: event.target.value } : candidate
                  )
                )
              }
            />
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label="Remove key"
              data-testid={`settings-mcp-servers-editor-secret-remove-${index}`}
              onClick={() => {
                rowKeys.remove(index);
                onChange(secretEnv.filter((_, i) => i !== index));
              }}
            >
              <Trash2 />
            </Button>
          </div>
        ))}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="self-start"
          data-testid="settings-mcp-servers-editor-secret-add"
          onClick={() => {
            rowKeys.append();
            onChange([...secretEnv, { key: "", binding: emptyBinding() }]);
          }}
        >
          <Plus />
          Add key
        </Button>
      </div>
    </div>
  );
}

function emptyBinding(): MCPSecretBinding {
  return { mode: "typed", existing: false, typedValue: "", vaultRef: "" };
}
