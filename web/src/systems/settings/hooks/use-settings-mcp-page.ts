import { useState } from "react";

import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";
import { deriveMCPManagementFilter, mcpDefinitionKey } from "../lib/mcp-management-target";
import type { SettingsMCPServerEntry, SettingsMCPServerListFilter } from "../types";
import { useMCPDefinitionAuthorization } from "./use-mcp-definition-authorization";
import { useMCPEditor } from "./use-mcp-editor";
import { useMCPOverrideEditor } from "./use-mcp-override-editor";
import { useSettingsMCPServers } from "./use-settings-collections";
import { useDeleteSettingsMCPServer } from "./use-settings-mutations";
import { useSettingsPage } from "./use-settings-page";

export function useSettingsMCPPage() {
  const page = useSettingsPage({ currentSlug: "mcp" });
  const workspace = useActiveWorkspace();
  const { destination: profile } = useProfileReadScope();
  const [workspaceId, setWorkspaceId] = useState<string | null>(null);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [removing, setRemoving] = useState<SettingsMCPServerEntry | null>(null);
  const filter: SettingsMCPServerListFilter =
    profile !== "default"
      ? { scope: "profile", profile, ...(workspaceId ? { workspace_id: workspaceId } : {}) }
      : workspaceId
        ? { scope: "workspace", workspace_id: workspaceId }
        : { scope: "user" };
  const collection = useSettingsMCPServers(filter);
  const servers = collection.data?.mcp_servers ?? [];
  const editor = useMCPEditor({
    enabled: true,
    scope: filter.scope ?? "user",
    servers,
    workspaceId,
    profileName: profile,
  });
  const override = useMCPOverrideEditor();
  const authorization = useMCPDefinitionAuthorization();
  const removal = useDeleteSettingsMCPServer();
  const selected = servers.find(server => mcpDefinitionKey(server) === selectedKey) ?? null;
  const normalized = query.trim().toLowerCase();
  const filteredServers = servers.filter(server =>
    [server.name, server.owner, server.runtime_name].some(value =>
      value?.toLowerCase().includes(normalized)
    )
  );
  const edit = (entry: SettingsMCPServerEntry) => {
    if (entry.owner?.startsWith("extension:")) override.openEdit(entry);
    else editor.openEdit(entry);
  };
  const authorize = (entry: SettingsMCPServerEntry) => {
    const target = deriveMCPManagementFilter(entry);
    if (target) authorization.authorize.requestAuthorize(target, entry);
  };
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
    page,
    collection,
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
    openCreate: editor.openCreate,
    editorProps: editor.editorProps,
    overrideProps: override.editorProps,
    removing,
    requestRemove,
    remove,
    removal,
    closeRemove: () => {
      if (!removal.isPending) setRemoving(null);
    },
  };
}
