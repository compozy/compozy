import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  emptyDraft,
  settingsMCPServersListOptions,
  validateDraft,
  type MCPDraft,
} from "@/systems/settings";
import { vaultSecretsListOptions } from "@/systems/vault";
import { useProfileReadScope } from "@/systems/profiles";
import type { AgentPayload, UpdateAgentParams } from "../types";

export type ServerInput = NonNullable<UpdateAgentParams["agent"]["mcp_servers"]>[number];

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
  const [mode, setMode] = useState<"pick" | "new">("pick");
  const [draft, setDraft] = useState<MCPDraft>(() => emptyDraft());
  const [selected, setSelected] = useState("");
  const [headers, setHeaders] = useState<{ id: string; key: string; value: string }[]>([]);
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
  const validHeaders = headers.every(header => header.key.trim() && header.value.trim());

  return {
    mode,
    setMode,
    draft,
    setDraft,
    selected,
    setSelected,
    headers,
    setHeaders,
    servers,
    vault,
    available,
    validation,
    nameConflict,
    errors,
    validHeaders,
    agent,
    onAdd,
    onClose,
  };
}
