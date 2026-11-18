import { useState } from "react";

import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";
import { deriveMCPManagementFilter, mcpDefinitionKey } from "../lib/mcp-management-target";
import type { SettingsMCPServerEntry, SettingsMCPServerListFilter } from "../types";
import { useMCPDefinitionAuthorization } from "./use-mcp-definition-authorization";
import { useMCPEditor } from "./use-mcp-editor";
import { useMCPRuntimeManagement } from "./use-mcp-runtime-management";
import { useMCPServerCheck } from "./use-mcp-server-check";
import { useMCPOverrideEditor } from "./use-mcp-override-editor";
import { useSettingsMCPServers } from "./use-settings-collections";
import { useDeleteSettingsMCPServer } from "./use-settings-mutations";
import { useSettingsPage } from "./use-settings-page";

interface UseMCPServerManagementOptions {
  /** Off while the hosting surface is hidden; the editor still opens. */
  enabled?: boolean;
  /** A host that owns the search (a route `?q=`) passes it; otherwise the hook keeps its own. */
  search?: { query: string; setQuery: (query: string) => void };
}

/**
 * Every MCP definition the daemon resolves for the shell's profile lens (Personal or one project),
 * with the row actions, editors, sign-in and removal Settings › MCP servers offers. Hosts render
 * it through `MCPServersManagementBody` + `MCPServersManagementDialogs`.
 */
export function useMCPServerManagement({
  enabled = true,
  search,
}: UseMCPServerManagementOptions = {}) {
  const workspace = useActiveWorkspace();
  const { destination: profile } = useProfileReadScope();
  const [workspaceId, setWorkspaceId] = useState<string | null>(null);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [ownQuery, setOwnQuery] = useState("");
  const query = search?.query ?? ownQuery;
  const setQuery = search?.setQuery ?? setOwnQuery;
  const filter = mcpListFilter(profile, workspaceId);
  const collection = useSettingsMCPServers(filter, { enabled });
  const runtime = useMCPRuntimeManagement({
    enabled,
    filter: {
      scope: filter.scope,
      workspace_id: filter.workspace_id,
      profile: filter.profile,
    },
  });
  const serverCheck = useMCPServerCheck();
  const servers = collection.data?.mcp_servers ?? [];
  // A create checks names against a list refetched when the editor opens, never a stale cache.
  // `fetchStatus` stays `paused` (not fetching) while offline, so only `idle` proves it settled.
  const serversReady =
    collection.data !== undefined && collection.fetchStatus === "idle" && !collection.isError;
  const editor = useMCPEditor({
    enabled: true,
    scope: filter.scope ?? "user",
    servers,
    workspaceId,
    profileName: profile,
    serversReady,
    project: mcpEditorProject(workspaceId, workspace.workspaces ?? [], workspace.runtimeWorkspace),
    serversError: collectionError(collection),
  });
  const openCreate = () => {
    void collection.refetch();
    editor.openCreate();
  };

  const override = useMCPOverrideEditor();
  const authorization = useMCPDefinitionAuthorization({ enabled });
  const selected = servers.find(server => mcpDefinitionKey(server) === selectedKey) ?? null;
  const filteredServers = filterMCPServers(servers, query);
  const edit = (entry: SettingsMCPServerEntry) => {
    if (entry.owner?.startsWith("extension:")) override.openEdit(entry);
    else editor.openEdit(entry);
  };
  const authorize = (entry: SettingsMCPServerEntry, trigger: HTMLElement) => {
    const target = deriveMCPManagementFilter(entry);
    if (target) authorization.authorize.requestAuthorize(target, entry, trigger);
  };
  const removal = useMCPServerRemoval();
  return {
    collection,
    runtime,
    serverCheck,
    servers,
    filteredServers,
    selected,
    query,
    setQuery,
    selectedKey: selected ? mcpDefinitionKey(selected) : undefined,
    select: (entry: SettingsMCPServerEntry) => setSelectedKey(mcpDefinitionKey(entry)),
    workspaceId,
    selectWorkspace: setWorkspaceId,
    workspaces: workspace.workspaces ?? [],
    profile,
    filter,
    edit,
    authorize,
    authorization,
    openCreate,
    editorProps: editor.editorProps,
    overrideProps: override.editorProps,
    ...removal,
  };
}

/** The list the page shows: the named profile's, else the picked project's, else the user's. */
function mcpListFilter(profile: string, workspaceId: string | null): SettingsMCPServerListFilter {
  if (profile !== "default") {
    return { scope: "profile", profile, ...(workspaceId ? { workspace_id: workspaceId } : {}) };
  }
  return workspaceId ? { scope: "workspace", workspace_id: workspaceId } : { scope: "user" };
}

function collectionError(collection: {
  isError: boolean;
  isFetching: boolean;
  error: Error | null;
  refetch: () => Promise<unknown>;
}): { message: string; retry: () => void } | null {
  if (!collection.isError || collection.isFetching || !collection.error) return null;
  return { message: collection.error.message, retry: () => void collection.refetch() };
}

function filterMCPServers(
  servers: readonly SettingsMCPServerEntry[],
  query: string
): SettingsMCPServerEntry[] {
  const normalized = query.trim().toLowerCase();
  return servers.filter(server =>
    [server.name, server.owner, server.runtime_name].some(value =>
      value?.toLowerCase().includes(normalized)
    )
  );
}

/** Remove confirmation for a user-owned server; extension servers are never removable here. */
function useMCPServerRemoval() {
  const removal = useDeleteSettingsMCPServer();
  const [removing, setRemoving] = useState<SettingsMCPServerEntry | null>(null);
  const requestRemove = (entry: SettingsMCPServerEntry) => {
    if (removal.isPending || entry.owner?.startsWith("extension:")) return;
    removal.reset();
    setRemoving(entry);
  };
  const remove = () => {
    if (!removing || removal.isPending) return;
    const target = deriveMCPManagementFilter(removing);
    if (!target || target.owner?.startsWith("extension:")) return;
    const identity = mcpDefinitionKey(removing);
    removal.mutate(
      { name: removing.name, filter: target },
      {
        onSuccess: () =>
          setRemoving(current =>
            current && mcpDefinitionKey(current) === identity ? null : current
          ),
      }
    );
  };
  return {
    removing,
    requestRemove,
    remove,
    removal,
    closeRemove: () => {
      if (!removal.isPending) setRemoving(null);
    },
  };
}

/** The page's project, else the open one: what "Only <project>" names in the editor. */
function mcpEditorProject(
  workspaceId: string | null,
  workspaces: readonly { id: string; name: string }[],
  open: { id: string; name: string } | undefined
) {
  if (workspaceId) {
    const listed = workspaces.find(candidate => candidate.id === workspaceId);
    return { id: workspaceId, name: listed?.name ?? workspaceId };
  }
  return open ? { id: open.id, name: open.name } : null;
}

export type MCPServerManagement = ReturnType<typeof useMCPServerManagement>;

/** Settings › MCP servers: the shared management model plus the Settings page/restart model. */
export function useSettingsMCPPage() {
  const page = useSettingsPage({ currentSlug: "mcp" });
  return { ...useMCPServerManagement(), page };
}
