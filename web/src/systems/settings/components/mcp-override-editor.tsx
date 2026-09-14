import { Cable, Plus, RotateCcw, Trash2 } from "lucide-react";
import { useId } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  FormSection,
  ImmutableIdentity,
  Input,
} from "@compozy/ui";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type {
  MCPOverrideDraft,
  MCPOverrideErrors,
  MCPOverridePair,
} from "../lib/mcp-override-model";
import { mcpManagementScopeLabel } from "../lib/mcp-management-target";
import type { SettingsMCPServerEntry } from "../types";

import { MCPFieldLabel } from "./mcp-editor-fields";

export interface MCPOverrideEditorProps {
  open: boolean;
  /** The extension-provided definition being overridden; never a manual server. */
  entry: SettingsMCPServerEntry;
  /** Full definition identity (owner · name · scope), stable across same-name rows. */
  identity: string;
  draft: MCPOverrideDraft;
  errors: MCPOverrideErrors;
  isValid: boolean;
  isSaving: boolean;
  saveError: string | null;
  onChange: (draft: MCPOverrideDraft) => void;
  onClose: () => void;
  onSave: () => void;
  /** Removes the stored override; the extension and its package stay untouched. */
  onReset: () => void;
}

/**
 * Edit configuration for an extension-provided MCP server. The package declaration — name,
 * owner, runtime name, command or URL, auth — is read-only identity; the dialog edits only the
 * override record the daemon applies on top of it: environment for a local process, headers and
 * an optional endpoint for a remote one. A blank URL inherits the manifest. Reset deletes the
 * override, never the extension.
 */
export function MCPOverrideEditor({
  open,
  entry,
  identity,
  draft,
  errors,
  isValid,
  isSaving,
  saveError,
  onChange,
  onClose,
  onSave,
  onReset,
}: MCPOverrideEditorProps) {
  if (!open) return null;
  const hasStoredOverride = Boolean(
    entry.override &&
    (Object.keys(entry.override.env ?? {}).length > 0 ||
      Object.keys(entry.override.headers ?? {}).length > 0 ||
      entry.override.url?.trim())
  );
  const ownerName = entry.owner?.replace(/^extension:/, "") ?? "";

  return (
    <Dialog
      open={open}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    >
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("md")}`}
        data-identity={identity}
        data-testid="settings-mcp-override-editor"
        data-transport={entry.transport}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            <>
              Provided by the <b className="font-medium text-muted">{ownerName}</b> extension.
              Values here are stored as an override on top of the package; the package itself never
              changes.
            </>
          }
          eyebrow="System · MCP server"
          icon={Cable}
          onClose={isSaving ? undefined : onClose}
          title={
            <span data-testid="settings-mcp-override-editor-title">
              Edit configuration · {entry.name}
            </span>
          }
        />

        <EntityDialogBody className="flex flex-col" data-testid="settings-mcp-override-editor-body">
          <MCPPackageDeclaration entry={entry} />
          <MCPOverrideFields
            entry={entry}
            draft={draft}
            errors={errors}
            isSaving={isSaving}
            onChange={onChange}
          />

          {saveError ? (
            <Alert data-testid="settings-mcp-override-editor-error" role="alert" variant="danger">
              <AlertDescription>{saveError}</AlertDescription>
            </Alert>
          ) : null}
        </EntityDialogBody>

        <EntityDialogFooter
          cancelDisabled={isSaving}
          cancelTestId="settings-mcp-override-editor-cancel"
          hint="Secrets never live here — declare them as extension inputs."
          isSaving={isSaving}
          leading={
            hasStoredOverride ? (
              <Button
                data-testid="settings-mcp-override-editor-reset"
                disabled={isSaving}
                onClick={onReset}
                size="sm"
                type="button"
                variant="ghost"
              >
                <RotateCcw aria-hidden="true" className="size-3" />
                Reset override
              </Button>
            ) : undefined
          }
          leadingTestId="settings-mcp-override-editor-leading"
          onCancel={onClose}
          onPrimary={onSave}
          primaryDisabled={!isValid || isSaving}
          primaryLabel={isSaving ? "Saving..." : "Save override"}
          primaryTestId="settings-mcp-override-editor-save"
        />
      </DialogContent>
    </Dialog>
  );
}

function MCPOverridePairsEditor({
  field,
  label,
  keyPlaceholder,
  addLabel,
  pairs,
  errors,
  disabled,
  onChange,
}: {
  field: "env" | "headers";
  label: string;
  keyPlaceholder: string;
  addLabel: string;
  pairs: MCPOverridePair[];
  errors?: Record<number, string>;
  disabled: boolean;
  onChange: (next: MCPOverridePair[]) => void;
}) {
  const rowKeys = useLocalRowKeys(pairs, `mcp-override-${field}`);
  const testPrefix = `settings-mcp-override-editor-${field}`;
  const noun = field === "env" ? "variable" : "header";
  return (
    <div>
      <MCPFieldLabel hint="name / value">{label}</MCPFieldLabel>
      <div className="flex flex-col gap-1.5" data-testid={`${testPrefix}-list`}>
        {pairs.length === 0 ? (
          <p className="text-form-hint text-subtle" data-testid={`${testPrefix}-empty`}>
            No override yet — the package values apply.
          </p>
        ) : null}
        {pairs.map((pair, index) => {
          const rowError = errors?.[index];
          const errorId = rowError ? `${rowKeys.keys[index]}-error` : undefined;
          return (
            <div key={rowKeys.keys[index]}>
              <div className="flex items-center gap-2">
                <Input
                  aria-describedby={errorId}
                  aria-invalid={rowError ? true : undefined}
                  aria-label={`${label} ${noun} name ${index + 1}`}
                  className="w-40 font-mono"
                  data-testid={`${testPrefix}-key-${index}`}
                  disabled={disabled}
                  onChange={event => {
                    const next = [...pairs];
                    next[index] = { ...pair, key: event.target.value };
                    onChange(next);
                  }}
                  placeholder={keyPlaceholder}
                  value={pair.key}
                />
                <Input
                  aria-describedby={errorId}
                  aria-invalid={rowError ? true : undefined}
                  aria-label={`${label} ${noun} value ${index + 1}`}
                  className="flex-1 font-mono"
                  data-testid={`${testPrefix}-value-${index}`}
                  disabled={disabled}
                  onChange={event => {
                    const next = [...pairs];
                    next[index] = { ...pair, value: event.target.value };
                    onChange(next);
                  }}
                  placeholder="value"
                  value={pair.value}
                />
                <Button
                  aria-label={`Remove ${noun} ${index + 1}`}
                  data-testid={`${testPrefix}-remove-${index}`}
                  disabled={disabled}
                  onClick={() => {
                    rowKeys.remove(index);
                    onChange(pairs.filter((_, i) => i !== index));
                  }}
                  size="icon-sm"
                  type="button"
                  variant="ghost"
                >
                  <Trash2 aria-hidden="true" className="size-3" />
                </Button>
              </div>
              {rowError ? (
                <p className="mt-1 text-caption text-danger" id={errorId} role="alert">
                  {rowError}
                </p>
              ) : null}
            </div>
          );
        })}
        <Button
          className="self-start"
          data-testid={`${testPrefix}-add`}
          disabled={disabled}
          onClick={() => {
            rowKeys.append();
            onChange([...pairs, { key: "", value: "" }]);
          }}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Plus aria-hidden="true" className="size-3" />
          {addLabel}
        </Button>
      </div>
    </div>
  );
}

function MCPPackageDeclaration({ entry }: Pick<MCPOverrideEditorProps, "entry">) {
  const isRemote = entry.transport !== "stdio";
  const scopeLabel = mcpManagementScopeLabel(entry) ?? entry.scope;
  return (
    <FormSection
      description="These fields come from the extension package and cannot be edited here."
      title="Package declaration"
    >
      <ImmutableIdentity
        data-testid="settings-mcp-override-editor-identity"
        rows={[
          { label: "Server", mono: true, value: entry.name },
          { label: "Owner", mono: true, value: entry.owner ?? "" },
          ...(entry.runtime_name?.trim()
            ? [{ label: "Runtime name", mono: true, value: entry.runtime_name }]
            : []),
          { label: "Scope", mono: true, value: scopeLabel },
          isRemote
            ? { label: "URL", mono: true, value: entry.url ?? "—" }
            : {
                label: "Command",
                mono: true,
                value: [entry.command, ...(entry.args ?? [])].filter(Boolean).join(" ") || "—",
              },
          { label: "Auth", value: entry.auth ? "OAuth (from the package)" : "None" },
        ]}
      />
    </FormSection>
  );
}
function MCPOverrideFields({
  entry,
  draft,
  errors,
  isSaving,
  onChange,
}: Pick<MCPOverrideEditorProps, "entry" | "draft" | "errors" | "isSaving" | "onChange">) {
  const urlId = useId();
  const isRemote = entry.transport !== "stdio";
  return (
    <FormSection
      description={
        isRemote
          ? "Headers are sent on every request. Leave the endpoint blank to keep the package URL."
          : "Environment variables are added to the process the package declares."
      }
      rightLabel={isRemote ? "remote endpoint" : "local process"}
      title="Override"
    >
      {isRemote ? (
        <>
          <div>
            <MCPFieldLabel hint="optional" htmlFor={urlId}>
              Endpoint
            </MCPFieldLabel>
            <Input
              aria-describedby={errors.url ? `${urlId}-error` : undefined}
              aria-invalid={errors.url ? true : undefined}
              className="font-mono"
              data-testid="settings-mcp-override-editor-url"
              disabled={isSaving}
              id={urlId}
              onChange={event => onChange({ ...draft, url: event.target.value })}
              placeholder={entry.url ?? "https://"}
              value={draft.url}
            />
            {errors.url ? (
              <p className="mt-1.5 text-caption text-danger" id={`${urlId}-error`} role="alert">
                {errors.url}
              </p>
            ) : null}
          </div>
          <MCPOverridePairsEditor
            addLabel="Add header"
            disabled={isSaving}
            errors={errors.headers}
            field="headers"
            keyPlaceholder="Header"
            label="Headers"
            pairs={draft.headers}
            onChange={headers => onChange({ ...draft, headers })}
          />
        </>
      ) : (
        <MCPOverridePairsEditor
          addLabel="Add variable"
          disabled={isSaving}
          errors={errors.env}
          field="env"
          keyPlaceholder="KEY"
          label="Environment"
          pairs={draft.env}
          onChange={env => onChange({ ...draft, env })}
        />
      )}
    </FormSection>
  );
}
