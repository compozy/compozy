import { Plug } from "lucide-react";

import { Empty, ListingRow, Pill, Section, cn } from "@agh/ui";

import type { AgentMCPServer, AgentPayload } from "../types";

export interface AgentMcpServersPanelProps {
  agent: AgentPayload;
  className?: string;
  /** When true, omit the section chrome (used inside Configuration tab Section). */
  bare?: boolean;
}

function envKeyNames(server: AgentMCPServer): string[] {
  return Object.keys(server.env ?? {});
}

function secretKeyNames(server: AgentMCPServer): string[] {
  return Object.keys(server.secret_env ?? {});
}

export function AgentMcpServersPanel({
  agent,
  className,
  bare = false,
}: AgentMcpServersPanelProps) {
  const mcpServers = agent.mcp_servers ?? [];

  const list =
    mcpServers.length === 0 ? (
      <Empty
        icon={Plug}
        title="No MCP servers"
        description="This agent does not declare any MCP servers."
        data-testid="agent-mcp-empty"
        fill={false}
        className="px-4 py-8"
      />
    ) : (
      <ul data-testid="agent-mcp-list">
        {mcpServers.map(server => {
          const transport = server.transport ?? "stdio";
          const commandOrUrl = server.url ?? server.command;
          const envKeys = envKeyNames(server);
          const secretKeys = secretKeyNames(server);
          return (
            <li key={server.name} className="border-b border-line-soft last:border-b-0">
              <ListingRow
                className="border-b-0"
                interactive={false}
                data-testid={`agent-mcp-row-${server.name}`}
              >
                <ListingRow.Icon>
                  <Plug className="size-4" />
                </ListingRow.Icon>
                <ListingRow.Main>
                  <ListingRow.Name mono>
                    <ListingRow.Title>{server.name}</ListingRow.Title>
                  </ListingRow.Name>
                  {commandOrUrl ? (
                    <ListingRow.Description className="font-mono tracking-mono">
                      {commandOrUrl}
                    </ListingRow.Description>
                  ) : null}
                  {envKeys.length > 0 || secretKeys.length > 0 ? (
                    <ListingRow.Meta data-testid={`agent-mcp-keys-${server.name}`}>
                      {envKeys.map(key => (
                        <Pill key={`env:${key}`} mono size="sm" tone="neutral">
                          {key}
                        </Pill>
                      ))}
                      {secretKeys.map(key => (
                        <Pill key={`secret:${key}`} mono size="sm" tone="warning">
                          {key}
                        </Pill>
                      ))}
                    </ListingRow.Meta>
                  ) : null}
                </ListingRow.Main>
                <ListingRow.Trail>
                  <Pill
                    mono
                    size="sm"
                    tone="info"
                    data-testid={`agent-mcp-transport-${server.name}`}
                  >
                    {transport}
                  </Pill>
                </ListingRow.Trail>
              </ListingRow>
            </li>
          );
        })}
      </ul>
    );

  if (bare) {
    return (
      <div className={cn("min-w-0", className)} data-testid="agent-mcp-servers-panel">
        {list}
      </div>
    );
  }

  return (
    <Section
      label="MCP servers"
      className={cn("min-w-0", className)}
      data-testid="agent-mcp-servers-panel"
    >
      {list}
    </Section>
  );
}
