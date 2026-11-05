import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { AgentMCPAddDialog } from "../agent-mcp-add-dialog";
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeAll, afterEach, afterAll } from "vitest";

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
    const { rerender } = render(
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
    rerender(
      <AgentMcpServersPanel
        agent={{ ...agentWithMcp, mcp_servers: [{ name: "pending", reference: true }] }}
        readOnly
        pendingNames={["pending"]}
        onRemovePending={remove}
      />
    );
    expect(
      screen.queryByRole("button", { name: "Remove pending pending" })
    ).not.toBeInTheDocument();
    expect(remove).toHaveBeenCalledOnce();
  });
});

// The MCP attachment editor owns transport-dependent validation and lossless
// credential entry. HTTP is the only substituted boundary.
const server = setupServer(
  http.get("*/api/settings/mcp-servers", () => HttpResponse.json({ mcp_servers: [] })),
  http.get("*/api/vault/secrets", () => HttpResponse.json({ secrets: [] }))
);
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function renderAddDialog() {
  const onAdd = vi.fn();
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <AgentMCPAddDialog
        agent={{ ...agentWithMcp, mcp_servers: [] }}
        additions={[]}
        onAdd={onAdd}
        onClose={vi.fn()}
      />
    </QueryClientProvider>
  );
  fireEvent.click(screen.getByRole("button", { name: "Add new server" }));
  fireEvent.click(screen.getByTestId("settings-mcp-servers-editor-transport-remote"));
  fireEvent.change(screen.getByTestId("settings-mcp-servers-editor-name-input"), {
    target: { value: "docs" },
  });
  fireEvent.change(screen.getByTestId("settings-mcp-servers-editor-url-input"), {
    target: { value: "https://example.com/mcp" },
  });
  return onAdd;
}

describe("Agent MCP attachment credentials", () => {
  it("Should ignore hidden HTTP header rows when switching to stdio", () => {
    const onAdd = renderAddDialog();
    fireEvent.click(screen.getByRole("button", { name: "Add header credential" }));
    expect(screen.getByRole("button", { name: "Add server" })).toBeDisabled();
    fireEvent.click(screen.getByTestId("settings-mcp-servers-editor-transport-stdio"));
    fireEvent.change(screen.getByTestId("settings-mcp-servers-editor-command-input"), {
      target: { value: "docs-server" },
    });
    expect(screen.getByRole("button", { name: "Add server" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Add server" }));
    expect(onAdd).toHaveBeenCalledWith(expect.objectContaining({ command: "docs-server" }));
    expect(onAdd.mock.calls[0][0]).not.toHaveProperty("secret_header_values");
  });

  it("Should reject header names that repeat after trimming and case folding", () => {
    const onAdd = renderAddDialog();
    fireEvent.click(screen.getByRole("button", { name: "Add header credential" }));
    fireEvent.change(screen.getByLabelText("Header secret 1"), { target: { value: "first" } });
    fireEvent.click(screen.getByRole("button", { name: "Add header credential" }));
    fireEvent.change(screen.getByLabelText("Header name 2"), {
      target: { value: " authorization " },
    });
    fireEvent.change(screen.getByLabelText("Header secret 2"), { target: { value: "second" } });
    expect(screen.getByRole("alert")).toHaveTextContent("Header names must be unique");
    expect(screen.getByRole("button", { name: "Add server" })).toBeDisabled();
    expect(onAdd).not.toHaveBeenCalled();
  });
  it("Should reject invalid header names before staging and recover after correction", () => {
    const onAdd = renderAddDialog();
    fireEvent.click(screen.getByRole("button", { name: "Add header credential" }));
    fireEvent.change(screen.getByLabelText("Header name 1"), {
      target: { value: "Authorization:" },
    });
    fireEvent.change(screen.getByLabelText("Header secret 1"), { target: { value: "credential" } });
    expect(screen.getByRole("alert")).toHaveTextContent("valid HTTP header name");
    expect(screen.getByRole("button", { name: "Add server" })).toBeDisabled();
    expect(onAdd).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("Header name 1"), {
      target: { value: "Authorization" },
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Add server" }));
    expect(onAdd).toHaveBeenCalledWith(
      expect.objectContaining({ secret_header_values: { Authorization: "credential" } })
    );
  });
  it.each(["Content-Type", "Cookie", "MCP-Session-Id", "proxy-authorization", "set-cookie"])(
    "Should reject reserved header %s before staging",
    name => {
      const onAdd = renderAddDialog();
      fireEvent.click(screen.getByRole("button", { name: "Add header credential" }));
      fireEvent.change(screen.getByLabelText("Header name 1"), { target: { value: name } });
      fireEvent.change(screen.getByLabelText("Header secret 1"), {
        target: { value: "credential" },
      });
      expect(screen.getByRole("alert")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Add server" })).toBeDisabled();
      expect(onAdd).not.toHaveBeenCalled();
    }
  );

  it("Should reject Authorization while OAuth is enabled and recover when it is disabled", () => {
    const onAdd = renderAddDialog();
    fireEvent.click(screen.getByRole("button", { name: "Add header credential" }));
    fireEvent.change(screen.getByLabelText("Header secret 1"), { target: { value: "credential" } });
    fireEvent.click(screen.getByTestId("settings-mcp-servers-editor-mode-advanced"));
    fireEvent.click(screen.getByRole("switch", { name: "Use OAuth" }));
    expect(screen.getByRole("alert")).toHaveTextContent("Authorization is managed by OAuth");
    expect(screen.getByRole("button", { name: "Add server" })).toBeDisabled();
    expect(onAdd).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("switch", { name: "Use OAuth" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Add server" }));
    expect(onAdd).toHaveBeenCalledWith(
      expect.objectContaining({ secret_header_values: { Authorization: "credential" } })
    );
  });
});
