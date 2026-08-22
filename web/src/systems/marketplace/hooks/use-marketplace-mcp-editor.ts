import { useQuery } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { marketplaceMCPEditorLogic } from "./marketplace-mcp-editor-logic";
import type { MCPConfigScope } from "./marketplace-mcp-scope";
import {
  deriveMCPManagementFilter,
  emptyDraft,
  type MCPDraft,
  type MCPServerEditorProps,
  SettingsApiError,
  type SettingsMCPServerEntry,
  type SettingsMCPServerTarget,
  toDraft,
  toRequest,
  usePutSettingsMCPServer,
  validateDraft,
} from "@/systems/settings";
import { vaultSecretsListOptions } from "@/systems/vault";

interface UseMarketplaceMCPEditorOptions {
  enabled: boolean;
  scope: MCPConfigScope;
  servers: readonly SettingsMCPServerEntry[];
  workspaceId?: string | null;
  profileName?: string | null;
}

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  return error instanceof Error ? error.message : null;
}

function useMarketplaceMCPEditor({
  enabled,
  scope: createScope,
  servers,
  workspaceId,
  profileName,
}: UseMarketplaceMCPEditorOptions) {
  const putMutation = usePutSettingsMCPServer();
  const resetPutMutation = putMutation.reset;
  const editorLogic = useStore(marketplaceMCPEditorLogic);
  const editorFlow = useSelector(editorLogic, snapshot => snapshot.context);
  const editor = editorFlow.editor;

  const editorOpen = enabled && editor.mode !== "closed";
  const vaultQuery = useQuery({
    ...vaultSecretsListOptions({ namespace: "mcp" }),
    enabled: editorOpen,
  });

  const openCreate = () => {
    if (
      !enabled ||
      (createScope === "workspace" && !workspaceId) ||
      (createScope === "profile" && !profileName)
    )
      return;
    resetPutMutation();
    editorLogic.trigger.editorOpened({
      editor: {
        draft: emptyDraft("stdio"),
        mode: "create",
        scope: createScope,
        target: "auto",
        workspaceId: workspaceId ?? undefined,
        profileName: profileName ?? undefined,
      },
    });
  };

  const openEdit = (entry: SettingsMCPServerEntry) => {
    if (!enabled) return;
    const management = deriveMCPManagementFilter(entry);
    if (!management) return;
    resetPutMutation();
    editorLogic.trigger.editorOpened({
      editor: {
        draft: toDraft(entry),
        entry,
        mode: "edit",
        scope: management.scope,
        target: management.target,
        workspaceId: management.scope === "user" ? undefined : management.workspace_id,
        profileName: management.scope === "profile" ? management.profile : undefined,
      },
    });
  };

  const closeEditor = () => {
    if (editorFlow.pendingSaveAttempt !== null) return;
    editorLogic.trigger.editorDismissed();
    resetPutMutation();
  };

  const updateDraft = (updater: (draft: MCPDraft) => MCPDraft) => {
    if (editor.mode !== "closed") {
      editorLogic.trigger.draftChanged({ draft: updater(editor.draft) });
    }
  };

  const setEditorTarget = (target: SettingsMCPServerTarget) => {
    editorLogic.trigger.targetChanged({ target });
  };

  const validation = editor.mode === "closed" ? null : validateDraft(editor.draft);
  const editorName = editor.mode === "closed" ? "" : editor.draft.name.trim();
  const nameConflict =
    editor.mode === "create" &&
    editorName.length > 0 &&
    servers.some(server => {
      const management = deriveMCPManagementFilter(server);
      return (
        management?.scope === editor.scope && server.name.toLowerCase() === editorName.toLowerCase()
      );
    });
  const errors =
    validation === null
      ? {}
      : nameConflict
        ? { ...validation.errors, name: `An MCP server named "${editorName}" already exists.` }
        : validation.errors;
  const isValid = validation !== null && validation.valid && !nameConflict;

  const saveEditor = () => {
    if (editor.mode === "closed" || !isValid) return;
    if (editor.scope === "workspace" && !editor.workspaceId) return;
    if (editor.scope === "profile" && !editor.profileName) return;
    const filter =
      editor.scope === "workspace"
        ? { scope: editor.scope, workspace_id: editor.workspaceId, target: editor.target }
        : editor.scope === "profile"
          ? {
              scope: editor.scope,
              profile: editor.profileName,
              workspace_id: editor.workspaceId,
              target: editor.target,
            }
          : { scope: editor.scope, target: editor.target };
    const name = editor.draft.name.trim();
    const body = toRequest(editor.draft);
    editorLogic.trigger.saveRequested({
      name,
      execute: () =>
        putMutation.mutateAsync({
          body,
          filter,
          name,
        }),
    });
  };

  const vaultRefs = (vaultQuery.data ?? []).map(secret => secret.ref);
  const vaultInventory = vaultQuery.isLoading
    ? ({ status: "loading" } as const)
    : vaultQuery.error
      ? ({
          status: "error",
          message: errorMessage(vaultQuery.error) ?? "Vault inventory could not be loaded",
          retry: () => void vaultQuery.refetch(),
        } as const)
      : ({ status: "ready", refs: vaultRefs } as const);

  const editorProps: MCPServerEditorProps | null =
    editor.mode === "closed"
      ? null
      : {
          availableTargets:
            editor.mode === "edit" ? [editor.target] : ["auto", "config", "sidecar"],
          draft: editor.draft,
          entry: editor.mode === "edit" ? editor.entry : null,
          errors,
          isSaving: editorFlow.pendingSaveAttempt !== null,
          isValid,
          mode: editor.mode,
          onChange: updateDraft,
          onClose: closeEditor,
          onSave: saveEditor,
          onTargetChange: setEditorTarget,
          open: true,
          saveError: errorMessage(putMutation.error),
          scope: editor.scope,
          target: editor.target,
          vaultInventory,
          warnings: putMutation.data?.warnings,
        };

  return { editorProps, openCreate, openEdit };
}

export { useMarketplaceMCPEditor };
export type { MCPConfigScope } from "./marketplace-mcp-scope";
export type { UseMarketplaceMCPEditorOptions };
