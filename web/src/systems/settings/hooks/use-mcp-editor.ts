import { useQuery } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { mcpEditorLogic, type MCPEditorState } from "../stores/mcp-editor-store";
import { deriveMCPManagementFilter } from "../lib/mcp-management-target";
import {
  emptyDraft,
  toDraft,
  toRequest,
  validateDraft,
  type MCPDraft,
} from "../lib/mcp-editor-model";
import { SettingsApiError } from "../adapters/settings-api";
import { useSettingsMCPServers } from "./use-settings-collections";
import { usePutSettingsMCPServer } from "./use-settings-mutations";
import type { MCPServerEditorProps } from "../components/mcp-server-editor";
import type { MCPScopeChoice } from "../components/mcp-editor-fields";
import type {
  SettingsMCPServerEntry,
  SettingsLayeredScope,
  SettingsMCPServerTarget,
} from "../types";
import { vaultSecretsListOptions } from "@/systems/vault";

interface UseMCPEditorOptions {
  enabled: boolean;
  scope: SettingsLayeredScope;
  servers: readonly SettingsMCPServerEntry[];
  workspaceId?: string | null;
  profileName?: string | null;
  /**
   * Whether `servers` reflects the loaded collection. Create saves wait for it: an empty list
   * while loading would pass the duplicate-name check and the PUT would replace that server.
   */
  serversReady?: boolean;
  /** Why `servers` is not ready, with a retry the editor offers in place. */
  serversError?: { message: string; retry: () => void } | null;
  /**
   * The project a new server can be limited to ("Only <project>"): the page's project, or the
   * open project while the page lists user-scope servers ("All projects").
   */
  project?: { id: string; name: string } | null;
}

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  return error instanceof Error ? error.message : null;
}

function resolveMCPEditorWriteFilter(
  scope: SettingsLayeredScope,
  target: SettingsMCPServerTarget,
  workspaceId?: string | null,
  profileName?: string | null
) {
  if (scope === "user") return { scope, target };
  if (scope === "workspace") {
    return workspaceId ? { scope, target, workspace_id: workspaceId } : null;
  }
  return profileName
    ? { profile: profileName, scope, target, workspace_id: workspaceId ?? undefined }
    : null;
}

function useMCPEditor({
  enabled,
  scope: createScope,
  servers,
  workspaceId,
  profileName,
  serversReady = true,
  serversError = null,
  project = null,
}: UseMCPEditorOptions) {
  const putMutation = usePutSettingsMCPServer();
  const resetPutMutation = putMutation.reset;
  const editorLogic = useStore(mcpEditorLogic);
  const editorFlow = useSelector(editorLogic, snapshot => snapshot.context);
  const editor = editorFlow.editor;

  const editorOpen = enabled && editor.mode !== "closed";
  const vaultQuery = useQuery({
    ...vaultSecretsListOptions({ namespace: "mcp" }),
    enabled: editorOpen,
  });

  // A new server whose chosen scope differs from the page's list is checked against that
  // scope's own list, or a same-name server there would be overwritten by the PUT.
  const otherScope = editor.mode === "create" && editor.scope !== createScope ? editor.scope : null;
  const otherServers = useSettingsMCPServers(
    otherScope === "workspace"
      ? {
          scope: "workspace",
          workspace_id: editor.mode === "closed" ? undefined : editor.workspaceId,
        }
      : { scope: "user" },
    { enabled: enabled && otherScope !== null }
  );
  const nameCheck = otherScope
    ? {
        servers: otherServers.data?.mcp_servers ?? [],
        ready:
          otherServers.data !== undefined &&
          otherServers.fetchStatus === "idle" &&
          !otherServers.isError,
        error:
          otherServers.isError && !otherServers.isFetching
            ? { message: otherServers.error.message, retry: () => void otherServers.refetch() }
            : null,
      }
    : { servers, ready: serversReady, error: serversError };

  const openCreate = () => {
    if (!enabled || !resolveMCPEditorWriteFilter(createScope, "auto", workspaceId, profileName))
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
    if (!management?.target) return;
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

  const { errors, isValid } = mcpEditorValidation(editor, nameCheck.servers, nameCheck.ready);
  const scopeChoice = mcpScopeChoice(editor, createScope, project, scope =>
    editorLogic.trigger.scopeChanged(
      scope === "workspace" && project ? { scope, workspaceId: project.id } : { scope: "user" }
    )
  );

  const saveEditor = () => {
    if (editor.mode === "closed" || !isValid) return;
    const filter = resolveMCPEditorWriteFilter(
      editor.scope,
      editor.target,
      editor.workspaceId,
      editor.profileName
    );
    if (!filter) return;
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
          serversError: editor.mode === "create" ? nameCheck.error : null,
          scopeChoice,
          scope: editor.scope,
          target: editor.target,
          vaultInventory,
          warnings: putMutation.data?.warnings,
        };

  return { editorProps, openCreate, openEdit };
}

export { useMCPEditor };
export type { UseMCPEditorOptions };

function mcpEditorValidation(
  editor: MCPEditorState,
  servers: readonly SettingsMCPServerEntry[],
  serversReady: boolean
) {
  const validation = editor.mode === "closed" ? null : validateDraft(editor.draft);
  const editorName = editor.mode === "closed" ? "" : editor.draft.name.trim();
  const nameConflict =
    editor.mode === "create" &&
    editorName.length > 0 &&
    servers.some(server => {
      const management = deriveMCPManagementFilter(server);
      if (!management) return false;
      return (
        management.scope === editor.scope &&
        !management.owner?.startsWith("extension:") &&
        (management.scope === "user" || management.workspace_id === editor.workspaceId) &&
        (management.scope !== "profile" || management.profile === editor.profileName) &&
        server.name.toLowerCase() === editorName.toLowerCase()
      );
    });
  const errors =
    validation === null
      ? {}
      : nameConflict
        ? { ...validation.errors, name: `An MCP server named "${editorName}" already exists.` }
        : validation.errors;
  const isValid =
    validation !== null &&
    validation.valid &&
    !nameConflict &&
    (editor.mode !== "create" || serversReady);
  return { errors, isValid };
}

/**
 * "All projects" or "Only <project>": a choice while creating from the user or project list with
 * a project known, and a read-only fact on an existing user or project server. Profile and agent
 * servers show neither.
 */
function mcpScopeChoice(
  editor: MCPEditorState,
  createScope: SettingsLayeredScope,
  project: { id: string; name: string } | null,
  onChange: (scope: "user" | "workspace") => void
): MCPScopeChoice | undefined {
  if (editor.mode === "closed") return undefined;
  if (editor.mode === "create") {
    if (createScope === "profile" || !project) return undefined;
    return {
      value: editor.scope === "workspace" ? "workspace" : "user",
      projectName: project.name,
      onChange,
    };
  }
  if (editor.scope !== "user" && editor.scope !== "workspace") return undefined;
  const projectName = project && editor.workspaceId === project.id ? project.name : "this project";
  return { value: editor.scope, projectName, disabled: true };
}
