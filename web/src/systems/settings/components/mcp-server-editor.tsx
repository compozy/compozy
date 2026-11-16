import { useState } from "react";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";
import { Plug, Plus, Trash2 } from "@compozy/ui/icons";

import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  type EntityMode,
} from "@compozy/ui";

import type { MCPDraft, MCPDraftErrors, MCPTransport } from "../lib/mcp-editor-model";
import { withTransport } from "../lib/mcp-editor-model";
import { mcpSourceLine } from "../lib/mcp-editor-paste";
import type {
  SettingsLayeredScope,
  SettingsMCPServerEntry,
  SettingsMCPServerTarget,
} from "../types";

import { MCPEditorConnectionSection, type MCPRequiredField } from "./mcp-editor-connection-section";
import { MCPNameSection, type MCPScopeChoice } from "./mcp-editor-fields";
import { MCPEditorMoreOptions } from "./mcp-editor-more-options";
import { MCPEditorKeysSection, type MCPVaultInventory } from "./mcp-secret-binding";

export interface MCPServerEditorProps {
  /** Host-owned fields (the agent picker's header credentials), shown after the keys. */
  children?: React.ReactNode;
  open: boolean;
  mode: "create" | "edit";
  draft: MCPDraft;
  scope: SettingsLayeredScope;
  errors: MCPDraftErrors;
  isValid: boolean;
  isSaving: boolean;
  saveError: string | null;
  /** Why Save waits (the name check's list is still loading); shown beside the disabled Save. */
  savePending?: string;
  /** Create only: the server list the name check needs could not load; retry keeps the draft. */
  serversError?: { message: string; retry: () => void } | null;
  warnings?: string[];
  vaultInventory: MCPVaultInventory;
  target: SettingsMCPServerTarget;
  availableTargets: SettingsMCPServerTarget[];
  entry: SettingsMCPServerEntry | null;
  /** "All projects" or "Only <project>"; omitted where scope isn't the person's choice. */
  scopeChoice?: MCPScopeChoice;
  /** Runtime status line shown under More options. */
  statusLabel?: string;
  /** `advanced` opens More options on mount. */
  initialTier?: EntityMode;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
  onTargetChange: (target: SettingsMCPServerTarget) => void;
  onClose: () => void;
  onSave: () => void;
  onRemove?: () => void;
}

/**
 * Add or edit an MCP server on the entity-dialog kit: name and where to use it, how it connects
 * (with a preview of what is added), the keys it needs, then a framed "More options" fold that
 * holds every advanced field. It keeps its own `Dialog` rather than `SettingsEditorDialog`
 * because the body depends on the transport.
 */
export function MCPServerEditor({
  children,
  open,
  mode,
  draft,
  scope,
  errors,
  isValid,
  isSaving,
  saveError,
  savePending,
  serversError,
  warnings,
  vaultInventory,
  target,
  availableTargets,
  entry,
  scopeChoice,
  statusLabel,
  initialTier = "simple",
  onChange,
  onTargetChange,
  onClose,
  onSave,
  onRemove,
}: MCPServerEditorProps) {
  const isCreate = mode === "create";
  const [touched, setTouched] = useState<ReadonlySet<MCPRequiredField>>(() => new Set());
  // Until the person picks Remote or Local, a new server's source line decides.
  const [picked, setPicked] = useState<MCPTransport | null>(() =>
    isCreate && mcpSourceLine(draft).trim() === "" ? null : draft.transport
  );
  // One identity per secret row, shared by "Key it needs" and the names under More options.
  const secretRowKeys = useLocalRowKeys(draft.secretEnv, "mcp-secret");
  if (!open) return null;

  const touch = (field: MCPRequiredField) => {
    setTouched(current => (current.has(field) ? current : new Set(current).add(field)));
  };
  // Switching transport clears the launch fields, so they start pristine again.
  const pick = (transport: MCPTransport) => {
    setPicked(transport);
    if (draft.transport === transport) return;
    setTouched(current => new Set([...current].filter(field => field === "name")));
    onChange(current => withTransport(current, transport));
  };
  // Required fields speak up only once touched; the disabled save still gates submission.
  const shownErrors: MCPDraftErrors = {
    ...errors,
    name: touched.has("name") ? errors.name : undefined,
    command: touched.has("command") ? errors.command : undefined,
    url: touched.has("url") ? errors.url : undefined,
  };

  return (
    <Dialog
      open={open}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    >
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-mode={mode}
        data-testid="settings-mcp-servers-editor"
        data-transport={draft.transport}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            isCreate
              ? "Give your agents new tools, like reading Linear issues or querying a database."
              : "Changes reach every session on its next message."
          }
          icon={Plug}
          onClose={isSaving ? undefined : onClose}
          title={
            <span data-testid="settings-mcp-servers-editor-title">
              {isCreate ? "Add an MCP server" : `Edit ${draft.name}`}
            </span>
          }
        />

        <EntityDialogBody className="flex flex-col" data-testid="settings-mcp-servers-editor-body">
          <MCPNameSection
            error={shownErrors.name}
            isCreate={isCreate}
            name={draft.name}
            onBlur={() => touch("name")}
            onChange={name => {
              touch("name");
              onChange(current => ({ ...current, name }));
            }}
            scopeChoice={scopeChoice}
          />
          <MCPEditorConnectionSection
            autoName={isCreate && !touched.has("name")}
            draft={draft}
            errors={shownErrors}
            isCreate={isCreate}
            onAdopt={setPicked}
            onChange={onChange}
            onPick={pick}
            onTouch={touch}
            picked={picked}
          />
          <MCPEditorKeysSection
            rowKeys={secretRowKeys}
            draft={draft}
            errors={errors}
            onChange={onChange}
            vaultInventory={vaultInventory}
          />
          {children ? <div className="mt-6 flex flex-col gap-3">{children}</div> : null}
          <MCPEditorMoreOptions
            availableTargets={availableTargets}
            defaultOpen={initialTier === "advanced"}
            draft={draft}
            entry={entry}
            errors={errors}
            onChange={onChange}
            onTargetChange={onTargetChange}
            projectName={scopeChoice?.projectName}
            scope={scope}
            secretRowKeys={secretRowKeys}
            statusLabel={statusLabel}
            target={target}
          />
          <MCPEditorAlerts saveError={saveError} serversError={serversError} warnings={warnings} />
        </EntityDialogBody>

        <EntityDialogFooter
          cancelDisabled={isSaving}
          cancelTestId="settings-mcp-servers-editor-cancel"
          hint={
            savePending ??
            (isCreate || !onRemove ? "Agents get its tools on their next message." : undefined)
          }
          hintTestId="settings-mcp-servers-editor-hint"
          isSaving={isSaving}
          leading={
            !isCreate && onRemove ? (
              <Button
                className="-ml-2 text-destructive"
                data-testid="settings-mcp-servers-editor-remove"
                disabled={isSaving}
                onClick={onRemove}
                size="sm"
                type="button"
                variant="ghost"
              >
                <Trash2 data-icon="inline-start" />
                Remove server
              </Button>
            ) : undefined
          }
          onCancel={onClose}
          onPrimary={onSave}
          primaryDisabled={!isValid || isSaving}
          primaryIcon={isCreate ? Plus : undefined}
          primaryLabel={isSaving ? "Saving..." : isCreate ? "Add server" : "Save changes"}
          primaryTestId="settings-mcp-servers-editor-save"
        />
      </DialogContent>
    </Dialog>
  );
}

function MCPEditorAlerts({
  saveError,
  serversError,
  warnings,
}: Pick<MCPServerEditorProps, "saveError" | "serversError" | "warnings">) {
  return (
    <>
      {serversError ? (
        <Alert
          className="mt-6"
          data-testid="settings-mcp-servers-editor-servers-error"
          variant="destructive"
        >
          <AlertDescription>
            Couldn't check existing server names. {serversError.message}
          </AlertDescription>
          <AlertActions>
            <Button onClick={serversError.retry} size="xs" type="button" variant="secondary">
              Try again
            </Button>
          </AlertActions>
        </Alert>
      ) : null}
      {saveError ? (
        <Alert
          className="mt-6"
          data-testid="settings-mcp-servers-editor-error"
          variant="destructive"
        >
          <AlertDescription>{saveError}</AlertDescription>
        </Alert>
      ) : null}
      {!saveError && warnings && warnings.length > 0 ? (
        <Alert
          className="mt-6"
          data-testid="settings-mcp-servers-editor-warnings"
          variant="warning"
        >
          <AlertDescription>
            <ul className="flex flex-col gap-1">
              {warnings.map(warning => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      ) : null}
    </>
  );
}
