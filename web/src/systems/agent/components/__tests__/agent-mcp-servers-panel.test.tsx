import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AgentMcpServersPanel } from "../agent-mcp-servers-panel";
import type { AgentPayload } from "../../types";

const agentWithMcp: AgentPayload = {
  label: "Coder",
  kind: "on_demand",
  resolved_appearance: {
    version: 1,
    avatar: "generated",
    seed: "coder",
    generation: 1,
    color: "#7c5cff",
  },
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
          label: "Coder",
          kind: "on_demand",
          resolved_appearance: {
            version: 1,
            avatar: "generated",
            seed: "coder",
            generation: 1,
            color: "#7c5cff",
          },
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
  it("Should offer attachment only for editable agents and surface a broken reference", () => {
    const onAdd = vi.fn();
    const { rerender } = render(
      <AgentMcpServersPanel
        agent={{
          ...agentWithMcp,
          mcp_servers: [{ name: "missing", reference: true, broken_reference: true }],
        }}
        readOnly={false}
        onAdd={onAdd}
      />
    );
    expect(screen.getByText("Broken reference")).toBeInTheDocument();
    screen.getByRole("button", { name: "Add MCP server" }).click();
    expect(onAdd).toHaveBeenCalledOnce();
    rerender(<AgentMcpServersPanel agent={agentWithMcp} readOnly onAdd={onAdd} />);
    expect(screen.queryByRole("button", { name: "Add MCP server" })).not.toBeInTheDocument();
  });
  it("Should remove a pending addition without offering removal for saved connections", () => {
    const remove = vi.fn();
    render(
      <AgentMcpServersPanel
        agent={{
          ...agentWithMcp,
          mcp_servers: [...(agentWithMcp.mcp_servers ?? []), { name: "pending", reference: true }],
        }}
        readOnly={false}
        pendingNames={["pending"]}
        onRemovePending={remove}
      />
    );
    screen.getByRole("button", { name: "Remove pending pending" }).click();
    expect(remove).toHaveBeenCalledWith("pending");
    expect(screen.queryByRole("button", { name: "Remove pending github" })).not.toBeInTheDocument();
  });
});
