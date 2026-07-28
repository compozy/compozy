import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AgentMcpServersPanel } from "../agent-mcp-servers-panel";
import type { AgentPayload } from "../../types";

const agentWithMcp: AgentPayload = {
  name: "coder",
  provider: "claude",
  prompt: "x",
  definition_digest: "d".repeat(64),
  origin: "workspace",
  mcp_servers: [
    {
      name: "github",
      transport: "stdio",
      command: "npx",
      env: {
        GITHUB_API_URL: "https://api.github.com",
      },
      secret_env: {
        GITHUB_TOKEN: "ghp_should_never_render",
      },
    },
  ],
};

describe("AgentMcpServersPanel", () => {
  it("Should render MCP key names only and never secret values", () => {
    render(<AgentMcpServersPanel agent={agentWithMcp} />);

    expect(screen.getByTestId("agent-mcp-row-github")).toHaveAttribute("data-slot", "listing-row");
    expect(screen.getByText("GITHUB_API_URL")).toBeInTheDocument();
    expect(screen.getByText("GITHUB_TOKEN")).toBeInTheDocument();
    expect(screen.queryByText("https://api.github.com")).not.toBeInTheDocument();
    expect(screen.queryByText("ghp_should_never_render")).not.toBeInTheDocument();
    expect(screen.getByTestId("agent-mcp-transport-github")).toHaveTextContent("stdio");
  });

  it("Should render empty state when no MCP servers are declared", () => {
    render(
      <AgentMcpServersPanel
        agent={{
          name: "coder",
          provider: "claude",
          prompt: "x",
          definition_digest: "d".repeat(64),
          origin: "global",
          mcp_servers: [],
        }}
      />
    );
    expect(screen.getByTestId("agent-mcp-empty")).toBeInTheDocument();
  });
});
