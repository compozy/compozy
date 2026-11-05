import { useState } from "react";
import { Cable } from "@compozy/ui/icons";

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
  EntityModeToolbar,
  type EntityMode,
} from "@compozy/ui";

import type { MCPDraft, MCPDraftErrors } from "../lib/mcp-editor-model";
import type {
  SettingsLayeredScope,
  SettingsMCPServerEntry,
  SettingsMCPServerTarget,
} from "../types";

import { MCPEditorConnectionSection } from "./mcp-editor-connection-section";
import { MCPEditorOAuthSection } from "./mcp-editor-oauth-section";
import { MCPEditorProcessSection } from "./mcp-editor-stdio-section";
import type { MCPVaultInventory } from "./mcp-secret-binding";
import { SettingsSourceBadge } from "./settings-source-badge";

export interface MCPServerEditorProps {
  children?: React.ReactNode;
  open: boolean;
  mode: "create" | "edit";
  draft: MCPDraft;
  scope: SettingsLayeredScope;
  errors: MCPDraftErrors;
  isValid: boolean;
  isSaving: boolean;
  saveError: string | null;
  /** Create only: the server list the name check needs could not load; retry keeps the draft. */
  serversError?: { message: string; retry: () => void } | null;
  warnings?: string[];
  vaultInventory: MCPVaultInventory;
  target: SettingsMCPServerTarget;
  availableTargets: SettingsMCPServerTarget[];
  entry: SettingsMCPServerEntry | null;
  /** Runtime status line for the toolbar trailing slot. */
  statusLabel?: string;
  initialTier?: EntityMode;
  onChange: (updater: (draft: MCPDraft) => MCPDraft) => void;
  onTargetChange: (target: SettingsMCPServerTarget) => void;
  onClose: () => void;
  onSave: () => void;
  onRemove?: () => void;
}

/**
 * MCP server editor on the shared entity-dialog shell.
 *
 * It keeps its own `Dialog` rather than routing through `SettingsEditorDialog`:
 * the mode toolbar and the transport-dependent body are not part of that shell's
 * contract, and nesting them would produce two competing chromes.
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
  serversError,
  warnings,
  vaultInventory,
  target,
  availableTargets,
  entry,
  statusLabel,
  initialTier = "simple",
  onChange,
  onTargetChange,
  onClose,
  onSave,
  onRemove,
}: MCPServerEditorProps) {
  const [tier, setTier] = useState<EntityMode>(initialTier);
  if (!open) return null;

  const isCreate = mode === "create";
  const isRemote = draft.transport !== "stdio";

  return (
    <Dialog
      open={open}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    >
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("md")}`}
        data-mode={mode}
        data-testid="settings-mcp-servers-editor"
        data-transport={draft.transport}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            isCreate ? (
              <>
                An MCP server gives agents extra{" "}
                <b className="font-medium text-muted-foreground">tools and resources</b> — a local
                process or a remote endpoint. Environment secrets stay write-only.
              </>
            ) : (
              <>
                Update the connection and rotate write-only environment secrets.{" "}
                <b className="font-medium text-muted-foreground">The route identity stays fixed.</b>
              </>
            )
          }
          eyebrow="System · MCP server"
          icon={Cable}
          onClose={isSaving ? undefined : onClose}
          title={
            <span data-testid="settings-mcp-servers-editor-title">
              {isCreate ? "Add MCP server" : `Edit ${draft.name}`}
            </span>
          }
        />

        <EntityModeToolbar
          mode={tier}
          onModeChange={setTier}
          testIdPrefix="settings-mcp-servers-editor"
          trailing={
            <span className="flex min-w-0 flex-wrap items-center gap-2">
              {/* The write target is owned by the caller, so scope is reported,
                  not chosen — but it must stay visible in both modes. */}
              <span
                className="font-mono text-form-label text-muted-foreground"
                data-testid="settings-mcp-servers-editor-scope"
              >
                MCP server · {scope}
              </span>
              {statusLabel ? (
                <span
                  className="font-mono text-form-label text-muted-foreground"
                  data-testid="settings-mcp-servers-editor-status"
                >
                  {statusLabel}
                </span>
              ) : null}
              {entry ? (
                <SettingsSourceBadge
                  data-testid="settings-mcp-servers-editor-source"
                  shadowed={entry.source_metadata.shadowed_sources ?? []}
                  source={entry.source_metadata.effective_source}
                />
              ) : null}
            </span>
          }
        />

        <EntityDialogBody className="flex flex-col" data-testid="settings-mcp-servers-editor-body">
          <MCPEditorConnectionSection
            availableTargets={availableTargets}
            draft={draft}
            errors={errors}
            isCreate={isCreate}
            onChange={onChange}
            onTargetChange={onTargetChange}
            scope={scope}
            target={target}
          />

          {tier === "advanced" && !isRemote ? (
            <MCPEditorProcessSection
              draft={draft}
              errors={errors}
              onChange={onChange}
              vaultInventory={vaultInventory}
            />
          ) : null}

          {tier === "advanced" && isRemote ? (
            <MCPEditorOAuthSection
              draft={draft}
              hasSavedOAuth={Boolean(entry?.auth)}
              onChange={onChange}
            />
          ) : null}

          {children}

          {serversError ? (
            <Alert data-testid="settings-mcp-servers-editor-servers-error" variant="destructive">
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
            <Alert data-testid="settings-mcp-servers-editor-error" variant="destructive">
              <AlertDescription>{saveError}</AlertDescription>
            </Alert>
          ) : null}
          {!saveError && warnings && warnings.length > 0 ? (
            <Alert data-testid="settings-mcp-servers-editor-warnings" variant="warning">
              <AlertDescription>
                <ul className="flex flex-col gap-1">
                  {warnings.map(warning => (
                    <li key={warning}>{warning}</li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          ) : null}
        </EntityDialogBody>

        <EntityDialogFooter
          cancelDisabled={isSaving}
          cancelTestId="settings-mcp-servers-editor-cancel"
          hint={
            !isCreate && onRemove ? (
              <Button
                data-testid="settings-mcp-servers-editor-remove"
                disabled={isSaving}
                onClick={onRemove}
                size="sm"
                type="button"
                variant="destructive"
              >
                Remove server
              </Button>
            ) : (
              "Secret values are never returned after save — only presence."
            )
          }
          isSaving={isSaving}
          onCancel={onClose}
          onPrimary={onSave}
          primaryDisabled={!isValid || isSaving}
          primaryLabel={isSaving ? "Saving..." : isCreate ? "Add server" : "Save changes"}
          primaryTestId="settings-mcp-servers-editor-save"
        />
      </DialogContent>
    </Dialog>
  );
}
