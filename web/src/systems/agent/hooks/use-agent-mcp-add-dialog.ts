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
  const headerNames = headers.flatMap(header => {
    const name = header.key.trim().toLowerCase();
    return name ? [name] : [];
  });
  const duplicateHeaders = new Set(headerNames).size !== headerNames.length;
  const headerError =
    draft.transport === "http" && duplicateHeaders ? "Header names must be unique." : null;
  const headerErrors = headers.map(header => {
    if (draft.transport !== "http") return null;
    if (header.key.trim() && !/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(header.key.trim()))
      return "Use a valid HTTP header name.";
    if (/[\r\n]/.test(header.value)) return "Remove line breaks from the header value.";
    return null;
  });
  const validHeaders =
    draft.transport !== "http" ||
    (!duplicateHeaders &&
      headers.every(
        (header, index) => header.key.trim() && header.value.trim() && !headerErrors[index]
      ));

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
    headerError,
    headerErrors,
    agent,
    onAdd,
    onClose,
  };
}
