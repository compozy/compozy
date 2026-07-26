import { Plug } from "lucide-react";

import { FormSection } from "@agh/ui";

import type { AgentPayload } from "../types";
import { AgentMcpServersPanel } from "./agent-mcp-servers-panel";

export function AgentSettingsMcpSection({ agent }: { agent: AgentPayload }) {
  return (
    <FormSection
      data-testid="agent-settings-mcp"
      icon={Plug}
      title="MCP servers"
      description="MCP servers are defined in the agent file."
    >
      <AgentMcpServersPanel agent={agent} bare />
    </FormSection>
  );
}
