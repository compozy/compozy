import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";

import {
  emptyDraft,
  deriveMCPManagementFilter,
  SettingsApiError,
  toDraft,
  toRequest,
  usePutSettingsMCPServer,
  validateDraft,
  type MCPDraft,
  type MCPServerEditorProps,
  type SettingsMCPServerEntry,
  type SettingsMCPServerTarget,
  type SettingsMutationResult,
} from "@/systems/settings";
import { vaultSecretsListOptions } from "@/systems/vault";

type MCPConfigScope = "global" | "workspace";

type MCPEditorState =
  | { mode: "closed" }
  | {
      draft: MCPDraft;
      mode: "create";
      scope: MCPConfigScope;
      target: SettingsMCPServerTarget;
      workspaceId?: string;
    }
  | {
      draft: MCPDraft;
      entry: SettingsMCPServerEntry;
      mode: "edit";
      scope: MCPConfigScope;
      target: SettingsMCPServerTarget;
      workspaceId?: string;
    };

interface UseMarketplaceMCPEditorOptions {
  enabled: boolean;
  scope: MCPConfigScope;
  servers: readonly SettingsMCPServerEntry[];
  workspaceId?: string | null;
}

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  return error instanceof Error ? error.message : null;
}

function mcpSaveMessage(name: string, result: SettingsMutationResult): string {
  const target = result.write_target ?? "write target unavailable";
  const lifecycle = result.restart_required ? "restart required" : "applied now";
  return `Saved "${name}" · ${target} · ${lifecycle}`;
}

function useMarketplaceMCPEditor({
  enabled,
  scope: createScope,
  servers,
  workspaceId,
}: UseMarketplaceMCPEditorOptions) {
  const putMutation = usePutSettingsMCPServer();
  const resetPutMutation = putMutation.reset;
  const [editor, setEditor] = useState<MCPEditorState>({ mode: "closed" });

  const editorOpen = enabled && editor.mode !== "closed";
  const vaultQuery = useQuery({
    ...vaultSecretsListOptions({ namespace: "mcp" }),
    enabled: editorOpen,
  });

  const openCreate = () => {
    if (!enabled || (createScope === "workspace" && !workspaceId)) return;
    resetPutMutation();
    setEditor({
      draft: emptyDraft("stdio"),
      mode: "create",
      scope: createScope,
      target: "auto",
      workspaceId: workspaceId ?? undefined,
    });
  };

  const openEdit = (entry: SettingsMCPServerEntry) => {
    if (!enabled) return;
    const management = deriveMCPManagementFilter(entry);
    if (!management) return;
    resetPutMutation();
    setEditor({
      draft: toDraft(entry),
      entry,
      mode: "edit",
      scope: management.scope,
      target: management.target,
      workspaceId: management.scope === "workspace" ? management.workspace_id : undefined,
    });
  };

  const closeEditor = () => {
    setEditor({ mode: "closed" });
    resetPutMutation();
  };

  const updateDraft = (updater: (draft: MCPDraft) => MCPDraft) => {
    setEditor(current =>
      current.mode === "closed" ? current : { ...current, draft: updater(current.draft) }
    );
  };

  const setEditorTarget = (target: SettingsMCPServerTarget) => {
    setEditor(current => (current.mode === "closed" ? current : { ...current, target }));
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
    const filter =
      editor.scope === "workspace"
        ? { scope: editor.scope, workspace_id: editor.workspaceId, target: editor.target }
        : { scope: editor.scope, target: editor.target };
    putMutation.mutate(
      {
        body: toRequest(editor.draft),
        filter,
        name: editor.draft.name.trim(),
      },
      {
        onSuccess: result => {
          toast.success(mcpSaveMessage(editor.draft.name.trim(), result));
          setEditor({ mode: "closed" });
        },
      }
    );
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
          isSaving: putMutation.isPending,
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
export type { MCPConfigScope, UseMarketplaceMCPEditorOptions };
