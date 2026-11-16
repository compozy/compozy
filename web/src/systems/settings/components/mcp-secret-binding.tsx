import { useState } from "react";

import { Button, FormSection, SecretField } from "@compozy/ui";
import { KeyRound } from "@compozy/ui/icons";

import {
  isSecretEnvUnset,
  type MCPDraft,
  type MCPDraftErrors,
  type MCPSecretEnvEntry,
} from "../lib/mcp-editor-model";

export type MCPVaultInventory =
  | { status: "loading" }
  | { status: "error"; message: string; retry: () => void }
  | { status: "ready"; refs: string[] };

export interface MCPSecretKeyFieldProps {
  entry: MCPSecretEnvEntry;
  index: number;
  error?: string;
  onBlur?: () => void;
  /** Explicit, query-backed `vault:mcp/**` inventory state. */
  vaultInventory: MCPVaultInventory;
  onChange: (entry: MCPSecretEnvEntry) => void;
}

/**
 * One write-only key on `SecretField`: a configured binding shows presence and is kept unless
 * replaced, a new value is typed once, or an existing Vault ref is bound instead. The bound
 * identifier and value are never read back.
 */
export function MCPSecretKeyField({
  entry,
  index,
  error,
  onBlur,
  vaultInventory,
  onChange,
}: MCPSecretKeyFieldProps) {
  const binding = entry.binding;
  const refs = vaultInventory.status === "ready" ? vaultInventory.refs : [];
  // Presence belongs to the original key; a renamed key needs a new value.
  const present =
    binding.existing &&
    (entry.originalKey === undefined || entry.key.trim() === entry.originalKey.trim());
  const update = (patch: Partial<MCPSecretEnvEntry["binding"]>) =>
    onChange({ ...entry, binding: { ...binding, ...patch } });

  return (
    <SecretField
      binding={{
        sources: refs.map(ref => ({ ref, present: true })),
        selectedRef: binding.vaultRef,
        onSelectRef: vaultRef => update({ mode: "ref", vaultRef }),
        loading: vaultInventory.status === "loading",
        error:
          vaultInventory.status === "error" ? (
            <span className="flex items-center gap-2">
              {vaultInventory.message}
              <Button onClick={vaultInventory.retry} size="xs" type="button" variant="ghost">
                Retry
              </Button>
            </span>
          ) : undefined,
        emptyLabel: "No keys saved in your vault for MCP servers yet.",
        namespaceLabel: "vault:mcp",
      }}
      editing={binding.mode !== "preserve"}
      error={error}
      id={`mcp-key-${index}`}
      label={<span className="font-mono">{entry.key.trim()}</span>}
      onBlur={onBlur}
      mode={binding.mode === "ref" ? "source" : "value"}
      onEditingChange={editing =>
        update(editing ? { mode: "typed", typedValue: "" } : { mode: "preserve", typedValue: "" })
      }
      onModeChange={mode =>
        update(
          mode === "source"
            ? { mode: "ref", vaultRef: binding.vaultRef || refs[0] || "" }
            : { mode: "typed" }
        )
      }
      onValueChange={typedValue => update({ mode: "typed", typedValue })}
      placeholder="Paste the key"
      present={present}
      presenceLabel="Saved in your vault"
      required
      sourceModeLabel="Use Vault"
      testIdPrefix={`settings-mcp-servers-editor-secret-value-${index}`}
      value={binding.typedValue}
    />
  );
}

/**
 * "Key it needs": every named secret of a local program, shown outside More options because the
 * server can't start without them. Names are managed under More options.
 */
export function MCPEditorKeysSection({
  draft,
  errors,
  vaultInventory,
  onChange,
}: {
  draft: MCPDraft;
  errors: MCPDraftErrors;
  vaultInventory: MCPVaultInventory;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
}) {
  const named = draft.secretEnv.flatMap((entry, index) =>
    entry.key.trim() ? [{ entry, index }] : []
  );
  // A missing value speaks up once its field was left; the count above says it from the start.
  const [left, setLeft] = useState<ReadonlySet<number>>(() => new Set());
  if (draft.transport !== "stdio" || named.length === 0) return null;
  const missing = named.filter(({ entry }) => isSecretEnvUnset(entry)).length;
  return (
    <FormSection
      data-testid="settings-mcp-editor-keys"
      description="Saved in your vault. Nobody sees it again, not even agents."
      icon={KeyRound}
      rightLabel={missing > 0 ? `${missing} required` : undefined}
      title={named.length === 1 ? "Key it needs" : "Keys it needs"}
    >
      {named.map(({ entry, index }) => (
        <MCPSecretKeyField
          entry={entry}
          error={left.has(index) ? errors.secretEnv?.[index] : undefined}
          index={index}
          onBlur={() => setLeft(current => new Set(current).add(index))}
          key={index}
          onChange={next =>
            onChange(current => ({
              ...current,
              secretEnv: current.secretEnv.map((candidate, i) => (i === index ? next : candidate)),
            }))
          }
          vaultInventory={vaultInventory}
        />
      ))}
    </FormSection>
  );
}
