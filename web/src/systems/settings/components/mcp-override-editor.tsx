import { KeyRound, Puzzle, RotateCcw } from "@compozy/ui/icons";
import { useId } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  dialogShellClass,
  Disclosure,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  FormSection,
  Input,
} from "@compozy/ui";

import type { MCPOverrideDraft, MCPOverrideErrors } from "../lib/mcp-override-model";
import { mcpManagementScopeLabel } from "../lib/mcp-management-target";
import type { SettingsMCPServerEntry } from "../types";

import { MCPOverridePairsEditor } from "./mcp-override-pairs-editor";
import { mcpOwnerExtensionName } from "./mcp-server-labels";

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
 * "Set up <server>" for an extension-provided MCP server. The package declaration — name, owner,
 * runtime name, command or URL, auth — is read-only and sits under "Technical details"; the
 * dialog edits only the override record the daemon applies on top of it: settings for a local
 * program, headers and an optional address for a remote one. A blank address inherits the
 * manifest. Reset deletes the override, never the extension.
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
  const extension = mcpOwnerExtensionName(entry.owner) ?? entry.owner ?? "";
  // The runtime reports when the server can't start as declared; nothing else claims "missing".
  const needsSetup = entry.runtime_status?.configured === false;

  return (
    <Dialog
      open={open}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    >
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-identity={identity}
        data-testid="settings-mcp-override-editor"
        data-transport={entry.transport}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            needsSetup
              ? `The ${extension} extension added this server. It needs something from you before sessions can use it.`
              : `The ${extension} extension added this server. What you set here sits on top of the extension's own settings.`
          }
          icon={Puzzle}
          onClose={isSaving ? undefined : onClose}
          title={<span data-testid="settings-mcp-override-editor-title">Set up {entry.name}</span>}
        />

        <EntityDialogBody className="flex flex-col" data-testid="settings-mcp-override-editor-body">
          <MCPOverrideFields
            draft={draft}
            entry={entry}
            errors={errors}
            isSaving={isSaving}
            needsSetup={needsSetup}
            onChange={onChange}
          />
          <MCPTechnicalDetails entry={entry} />
          {saveError ? (
            <Alert
              className="mt-6"
              data-testid="settings-mcp-override-editor-error"
              role="alert"
              variant="destructive"
            >
              <AlertDescription>{saveError}</AlertDescription>
            </Alert>
          ) : null}
        </EntityDialogBody>

        <EntityDialogFooter
          cancelDisabled={isSaving}
          cancelTestId="settings-mcp-override-editor-cancel"
          isSaving={isSaving}
          leading={
            hasStoredOverride ? (
              <Button
                className="-ml-2"
                data-testid="settings-mcp-override-editor-reset"
                disabled={isSaving}
                onClick={onReset}
                size="sm"
                type="button"
                variant="ghost"
              >
                <RotateCcw data-icon="inline-start" />
                Reset to extension defaults
              </Button>
            ) : undefined
          }
          leadingTestId="settings-mcp-override-editor-leading"
          onCancel={onClose}
          onPrimary={onSave}
          primaryDisabled={!isValid || isSaving}
          primaryLabel={isSaving ? "Saving..." : "Save"}
          primaryTestId="settings-mcp-override-editor-save"
        />
      </DialogContent>
    </Dialog>
  );
}

/** "What it needs": the override values, the only part of the server this dialog writes. */
function MCPOverrideFields({
  entry,
  draft,
  errors,
  isSaving,
  needsSetup,
  onChange,
}: Pick<MCPOverrideEditorProps, "entry" | "draft" | "errors" | "isSaving" | "onChange"> & {
  needsSetup: boolean;
}) {
  const urlId = useId();
  const isRemote = entry.transport !== "stdio";
  return (
    <FormSection
      data-testid="settings-mcp-override-editor-fields"
      description={
        isRemote
          ? "Headers go with every request. Secrets don't belong here; the extension asks for those itself."
          : "Settings are added to the program the extension runs. Secrets don't belong here; the extension asks for those itself."
      }
      icon={KeyRound}
      rightLabel={needsSetup ? "Not set up yet" : undefined}
      title="What it needs"
    >
      {isRemote ? (
        <>
          <Field data-invalid={errors.url ? "true" : undefined}>
            <FieldLabel htmlFor={urlId}>Address</FieldLabel>
            <Input
              aria-describedby={errors.url ? `${urlId}-error` : `${urlId}-description`}
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
              <FieldError id={`${urlId}-error`}>{errors.url}</FieldError>
            ) : (
              <FieldDescription id={`${urlId}-description`}>
                Leave it blank to use the extension's address.
              </FieldDescription>
            )}
          </Field>
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
          addLabel="Add setting"
          disabled={isSaving}
          errors={errors.env}
          field="env"
          keyPlaceholder="KEY"
          label="Settings"
          pairs={draft.env}
          onChange={env => onChange({ ...draft, env })}
        />
      )}
    </FormSection>
  );
}

/** The package declaration, read-only: where it runs, how agents call it, which version. */
function MCPTechnicalDetails({ entry }: { entry: SettingsMCPServerEntry }) {
  const address =
    entry.transport === "stdio"
      ? [entry.command, ...(entry.args ?? [])].filter(Boolean).join(" ")
      : (entry.url ?? "");
  const version =
    entry.catalog_entry && entry.catalog_version
      ? `${entry.catalog_entry}@${entry.catalog_version}`
      : entry.catalog_version;
  const rows: Array<[string, string]> = [
    [entry.transport === "stdio" ? "Command" : "Address", address || "—"],
    ["Agents call it", `mcp__${entry.runtime_name?.trim() || entry.name}__*`],
    ["Added by", entry.owner ?? ""],
    ["Scope", mcpManagementScopeLabel(entry) ?? entry.scope],
    ["Sign-in", entry.auth ? "OAuth, from the extension" : "None"],
    ...(version ? ([["Version", version]] as Array<[string, string]>) : []),
  ];
  return (
    <Disclosure className="mt-6" label="Technical details" variant="framed">
      <dl
        className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 p-4 text-meta"
        data-testid="settings-mcp-override-editor-identity"
      >
        {rows.map(([label, value]) => (
          <div className="contents" key={label}>
            <dt className="text-faint">{label}</dt>
            <dd className="truncate font-mono text-muted-foreground">{value}</dd>
          </div>
        ))}
      </dl>
    </Disclosure>
  );
}
