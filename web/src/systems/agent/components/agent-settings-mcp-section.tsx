import { Plug } from "@compozy/ui/icons";
import { FormSection } from "@compozy/ui";
import type { AgentPayload } from "../types";
import type { AgentSettingsDraft } from "../lib/agent-settings-draft";
import { AgentMcpServersPanel } from "./agent-mcp-servers-panel";
import { AgentMCPAddDialog } from "./agent-mcp-add-dialog";
import { useState } from "react";

export function AgentSettingsMcpSection({
  agent,
  draft,
  onPatch,
  readOnly,
  workspaceId,
}: {
  agent: AgentPayload;
  draft: AgentSettingsDraft;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
  readOnly: boolean;
  workspaceId?: string | null;
}) {
  const [adding, setAdding] = useState(false);
  const additions = draft.mcpAdditions ?? [];
  return (
    <FormSection
      data-testid="agent-settings-mcp"
      icon={Plug}
      title="MCP servers"
      help="Attach a configured server or add a connection for this agent."
    >
      <AgentMcpServersPanel
        agent={{
          ...agent,
          mcp_servers: [
            ...(agent.mcp_servers ?? []),
            ...additions.map(server => ({
              ...server,
              auth: server.auth
                ? {
                    ...server.auth,
                    client_secret_configured: Boolean(server.auth.client_secret_ref),
                  }
                : null,
            })),
          ],
        }}
        bare
        readOnly={readOnly}
        onAdd={() => setAdding(true)}
        pendingNames={additions.map(server => server.name)}
        onRemovePending={name =>
          onPatch({ mcpAdditions: additions.filter(server => server.name !== name) })
        }
      />
      {adding && !readOnly ? (
        <AgentMCPAddDialog
          agent={agent}
          workspaceId={workspaceId}
          additions={additions}
          onClose={() => setAdding(false)}
          onAdd={server => {
            onPatch({ mcpAdditions: [...additions, server] });
            setAdding(false);
          }}
        />
      ) : null}
    </FormSection>
  );
}
