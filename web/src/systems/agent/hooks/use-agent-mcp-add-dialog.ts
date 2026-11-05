import { useSelector, useStore } from "@xstate/store-react";
import { useQuery } from "@tanstack/react-query";
import { settingsMCPServersListOptions, validateDraft, type MCPDraft } from "@/systems/settings";
import { vaultSecretsListOptions } from "@/systems/vault";
import { useProfileReadScope } from "@/systems/profiles";
import type { AgentPayload, UpdateAgentParams } from "../types";
import { agentMCPAddDialogLogic } from "../stores/agent-mcp-add-dialog-store";

export type ServerInput = NonNullable<UpdateAgentParams["agent"]["mcp_servers"]>[number];

/** Binds one dialog store to scoped Query data and derives validation without storing it. */
export function useAgentMCPAddDialog({
  agent,
  additions,
  workspaceId,
  onAdd,
  onClose,
}: {
  agent: AgentPayload;
  additions: ServerInput[];
  workspaceId?: string | null;
  onAdd: (server: ServerInput) => void;
  onClose: () => void;
}) {
  const store = useStore(agentMCPAddDialogLogic);
  const { mode, draft, selected, headers } = useSelector(store, snapshot => snapshot.context);
  const { destination } = useProfileReadScope();
  const servers = useQuery(
    settingsMCPServersListOptions({
      ...(workspaceId ? { scope: "workspace", workspace_id: workspaceId } : {}),
      profile: destination,
    })
  );
  const vault = useQuery(vaultSecretsListOptions({ namespace: "mcp" }));
  const used = new Set([...(agent.mcp_servers ?? []), ...additions].map(server => server.name));
  const available = (servers.data?.mcp_servers ?? []).filter(
    server => (!server.owner || server.owner === "manual") && !used.has(server.name)
  );
  const validation = validateDraft(draft);
  const nameConflict = used.has(draft.name.trim());
  const errors = nameConflict
    ? { ...validation.errors, name: "This agent already has this server." }
    : validation.errors;
  const headerNames = headers.flatMap(header => {
    const name = header.key.trim().toLowerCase();
    return name ? [name] : [];
  });
  const duplicateHeaders = new Set(headerNames).size !== headerNames.length;
  const headerError =
    draft.transport === "http" && duplicateHeaders ? "Header names must be unique." : null;
  const headerErrors = headers.map(header =>
    draft.transport === "http" ? validateHeaderCredential(header, draft.oauth.enabled) : null
  );
  const validHeaders =
    draft.transport !== "http" ||
    (!duplicateHeaders &&
      headers.every(
        (header, index) => header.key.trim() && header.value.trim() && !headerErrors[index]
      ));

  return {
    mode,
    onNewServer: () => store.trigger.newServerRequested(),
    draft,
    onDraftChange: (update: (draft: MCPDraft) => MCPDraft) =>
      store.trigger.draftChanged({ draft: update(store.getSnapshot().context.draft) }),
    selected,
    onSelectServer: (name: string) => store.trigger.serverSelected({ name }),
    headers,
    onHeaderAdd: () => store.trigger.headerAdded({ id: crypto.randomUUID() }),
    onHeaderChange: (id: string, patch: { key?: string; value?: string }) =>
      store.trigger.headerChanged({ id, patch }),
    onHeaderRemove: (id: string) => store.trigger.headerRemoved({ id }),
    servers,
    vault,
    available,
    validation,
    nameConflict,
    errors,
    validHeaders,
    headerError,
    headerErrors,
    agent,
    onAdd,
    onClose,
  };
}

/** Mirrors the daemon header policy before a connection becomes a pending addition. */
function validateHeaderCredential(header: { key: string; value: string }, oauthEnabled: boolean) {
  const name = header.key.trim();
  if (name && !/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(name)) return "Use a valid HTTP header name.";
  const normalized = name.toLowerCase();
  if (normalized === "content-type" || normalized.startsWith("mcp-"))
    return "This header is managed by the MCP connection.";
  if (["cookie", "proxy-authorization", "set-cookie"].includes(normalized))
    return "Use Authorization for header credentials.";
  if (normalized === "authorization" && oauthEnabled) return "Authorization is managed by OAuth.";
  if (/[\r\n]/.test(header.value)) return "Remove line breaks from the header value.";
  return null;
}
