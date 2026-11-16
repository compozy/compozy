import { useSelector, useStore } from "@xstate/store-react";
import { useQuery } from "@tanstack/react-query";
import { settingsMCPServersListOptions, validateDraft, type MCPDraft } from "@/systems/settings";
import { vaultSecretsListOptions } from "@/systems/vault";
import { useProfileReadScope } from "@/systems/profiles";
import type { AgentPayload } from "../types";
import { agentMCPAddDialogLogic } from "../stores/agent-mcp-add-dialog-store";
import {
  agentMCPPickerRows,
  filterAgentMCPPickerRows,
  nextAgentMCPAdditions,
  type ServerInput,
} from "../lib/agent-mcp-picker-model";

export type { ServerInput };

/** Binds one dialog store to scoped Query data and derives validation without storing it. */
export function useAgentMCPAddDialog({
  agent,
  additions,
  workspaceId,
  onSave,
  onClose,
}: {
  agent: AgentPayload;
  additions: ServerInput[];
  workspaceId?: string | null;
  /** Receives the full next staged list: picker toggles plus any server created here. */
  onSave: (additions: ServerInput[]) => void;
  onClose: () => void;
}) {
  const store = useStore(agentMCPAddDialogLogic);
  const { mode, query, overrides, draft, headers } = useSelector(
    store,
    snapshot => snapshot.context
  );
  const { destination } = useProfileReadScope();
  const servers = useQuery(
    settingsMCPServersListOptions({
      ...(workspaceId ? { scope: "workspace", workspace_id: workspaceId } : {}),
      profile: destination,
    })
  );
  const vault = useQuery(vaultSecretsListOptions({ namespace: "mcp" }));
  const saved = new Set((agent.mcp_servers ?? []).map(server => server.name));
  const used = new Set([
    ...saved,
    ...additions.map(server => server.name),
    ...Object.keys(overrides).filter(name => overrides[name]),
  ]);
  const rows = agentMCPPickerRows({
    entries: servers.data?.mcp_servers ?? [],
    agent,
    additions,
    overrides,
  });
  const visibleRows = filterAgentMCPPickerRows(rows, query);
  const checkedCount = rows.filter(row => row.checked).length;
  const pickedAdditions = () => nextAgentMCPAdditions(additions, overrides, saved);
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
    onBackToPicker: () => store.trigger.pickerRequested(),
    query,
    onQueryChange: (next: string) => store.trigger.queryChanged({ query: next }),
    rows,
    visibleRows,
    checkedCount,
    onToggleServer: (name: string, checked: boolean) =>
      store.trigger.serverToggled({ name, checked }),
    headers,
    onHeaderAdd: () => store.trigger.headerAdded({ id: crypto.randomUUID() }),
    onHeaderChange: (id: string, patch: { key?: string; value?: string }) =>
      store.trigger.headerChanged({ id, patch }),
    onHeaderRemove: (id: string) => store.trigger.headerRemoved({ id }),
    servers,
    vault,
    validation,
    nameConflict,
    errors,
    validHeaders,
    headerError,
    headerErrors,
    agent,
    onSave: () => onSave(pickedAdditions()),
    onCreate: (server: ServerInput) => onSave([...pickedAdditions(), server]),
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
