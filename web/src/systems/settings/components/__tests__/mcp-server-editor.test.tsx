import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  emptyDraft,
  toDraft,
  toRequest,
  validateDraft,
  type MCPDraft,
} from "../../lib/mcp-editor-model";
import type { SettingsMCPServerEntry } from "../../types";
import { MCPServerEditor, type MCPServerEditorProps } from "../mcp-server-editor";

function makeProps(overrides: Partial<MCPServerEditorProps> = {}): MCPServerEditorProps {
  return {
    availableTargets: ["config"],
    draft: { ...emptyDraft("stdio"), name: "github", command: "bunx" },
    entry: null,
    errors: {},
    isSaving: false,
    isValid: true,
    mode: "create",
    onChange: vi.fn(),
    onClose: vi.fn(),
    onSave: vi.fn(),
    onTargetChange: vi.fn(),
    open: true,
    saveError: null,
    scope: "workspace",
    target: "config",
    vaultInventory: { status: "ready", refs: [] },
    ...overrides,
  };
}

/** Stateful host so transport switches and tier changes behave like production. */
function EditorHarness(overrides: Partial<MCPServerEditorProps> = {}) {
  const base = makeProps(overrides);
  const [draft, setDraft] = useState<MCPDraft>(base.draft);
  return (
    <>
      <MCPServerEditor {...base} draft={draft} onChange={updater => setDraft(updater)} />
      <output data-testid="mcp-draft-probe">{JSON.stringify(draft)}</output>
    </>
  );
}

/** Host that derives errors and validity from the draft, as the owning hooks do. */
function ValidatingEditorHarness({ draft: initial }: { draft: MCPDraft }) {
  const [draft, setDraft] = useState<MCPDraft>(initial);
  const validation = validateDraft(draft);
  return (
    <MCPServerEditor
      {...makeProps()}
      draft={draft}
      errors={validation.errors}
      isValid={validation.valid}
      onChange={updater => setDraft(updater)}
    />
  );
}

function readDraft(): MCPDraft {
  return JSON.parse(screen.getByTestId("mcp-draft-probe").textContent ?? "{}");
}

describe("MCPServerEditor", () => {
  it("Should let an explicit OAuth server with headers undo a toggle before saving", async () => {
    const user = userEvent.setup();
    const entry: SettingsMCPServerEntry = {
      approval: { required: false, state: "not_required", definition_digest: "sha256:test" },
      callable_in_sessions: true,
      name: "remote",
      transport: "http",
      url: "https://example.test/mcp",
      headers_configured: true,
      auth: { registration: "auto", client_secret_configured: false },
      scope: "workspace",
      source_metadata: {
        available_targets: ["workspace-config"],
        effective_source: { kind: "workspace-config", scope: "workspace" },
      },
    };
    render(
      <EditorHarness mode="edit" entry={entry} initialTier="advanced" draft={toDraft(entry)} />
    );
    const toggle = screen.getByRole("switch", { name: "Use OAuth" });
    await user.click(toggle);
    expect(toRequest(readDraft()).server.auth).toBeUndefined();
    expect(toggle).not.toHaveAttribute("aria-disabled", "true");
    await user.click(toggle);
    expect(toRequest(readDraft())).toMatchObject({
      server: { auth: { registration: "auto" } },
      preserve_headers: true,
    });
  });

  it("Should keep implicit OAuth disabled for a saved header-auth server", async () => {
    const user = userEvent.setup();
    const entry: SettingsMCPServerEntry = {
      approval: { required: false, state: "not_required", definition_digest: "sha256:test" },
      callable_in_sessions: true,
      name: "header",
      transport: "http",
      url: "https://example.test/mcp",
      headers_configured: true,
      scope: "workspace",
      source_metadata: {
        available_targets: ["workspace-config"],
        effective_source: { kind: "workspace-config", scope: "workspace" },
      },
    };
    render(
      <EditorHarness mode="edit" entry={entry} initialTier="advanced" draft={toDraft(entry)} />
    );
    const toggle = screen.getByRole("switch", { name: "Use OAuth" });
    expect(toggle).toHaveAttribute("aria-disabled", "true");
    await user.click(toggle);
    expect(toRequest(readDraft()).server.auth).toBeUndefined();
  });

  it("Should toggle OAuth in the saved remote definition", async () => {
    const user = userEvent.setup();
    render(
      <EditorHarness
        initialTier="advanced"
        draft={{ ...emptyDraft("http"), name: "remote", url: "https://example.test/mcp" }}
      />
    );
    await user.click(screen.getByRole("switch", { name: "Use OAuth" }));
    expect(toRequest(readDraft()).server.auth).toEqual({ registration: "auto" });
    await user.click(screen.getByRole("switch", { name: "Use OAuth" }));
    expect(toRequest(readDraft()).server.auth).toBeUndefined();
  });

  it("Should present one named dialog with a single primary action", () => {
    render(<MCPServerEditor {...makeProps()} />);

    expect(screen.getByRole("dialog", { name: "Add an MCP server" })).toBeInTheDocument();
    expect(screen.getByTestId("settings-mcp-servers-editor-save")).toHaveTextContent("Add server");
    expect(screen.getByTestId("settings-mcp-servers-editor-cancel")).toBeInTheDocument();
  });

  it("Should keep the source visible and fold the process environment under More options", async () => {
    const user = userEvent.setup();
    render(<EditorHarness />);

    expect(screen.getByTestId("settings-mcp-servers-editor-command-input")).toHaveValue("bunx");
    expect(screen.queryByTestId("settings-mcp-editor-stdio")).toBeNull();

    await user.click(screen.getByRole("button", { name: "More options" }));

    expect(screen.getByTestId("settings-mcp-editor-stdio")).toBeInTheDocument();
    // More options never hides the required connection fields.
    expect(screen.getByTestId("settings-mcp-servers-editor-command-input")).toBeInTheDocument();
  });

  it("Should name where a server is used in words under More options, not the scope value", async () => {
    const user = userEvent.setup();
    render(
      <EditorHarness
        scope="workspace"
        scopeChoice={{ value: "workspace", projectName: "compozy" }}
      />
    );

    await user.click(screen.getByRole("button", { name: "More options" }));
    expect(screen.getByTestId("settings-mcp-servers-editor-scope-fact")).toHaveTextContent(
      "Only compozy"
    );
    expect(screen.getByText("Used in")).toBeInTheDocument();
    expect(screen.queryByText("workspace")).toBeNull();
  });

  it("Should offer OAuth only for a remote transport", async () => {
    const user = userEvent.setup();
    render(<EditorHarness initialTier="advanced" />);

    expect(screen.queryByTestId("settings-mcp-editor-oauth")).toBeNull();

    await user.click(screen.getByTestId("settings-mcp-servers-editor-transport-remote"));

    expect(screen.getByTestId("settings-mcp-editor-oauth")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-mcp-editor-stdio")).toBeNull();
  });

  // Invariant: until a transport is picked, the one source field decides from what is typed —
  // a URL is a remote server, a line is a command split into command + args — and the name
  // follows the source until the person types one.
  it("Should detect the transport from the source line and suggest a name", async () => {
    const user = userEvent.setup();
    render(<EditorHarness draft={emptyDraft("stdio")} />);

    expect(screen.getByRole("radio", { name: "Remote server" })).toHaveAttribute(
      "aria-checked",
      "false"
    );
    expect(screen.getByLabelText("URL or command")).toBeInTheDocument();

    await user.type(screen.getByLabelText("URL or command"), "npx -y @acme/mcp-server-notes");
    expect(readDraft()).toMatchObject({
      transport: "stdio",
      command: "npx",
      args: ["-y", "@acme/mcp-server-notes"],
      name: "notes",
    });
    expect(screen.getByLabelText("Command")).toHaveValue("npx -y @acme/mcp-server-notes");
    expect(screen.getByTestId("settings-mcp-servers-editor-preview")).toHaveTextContent("notes");

    await user.clear(screen.getByLabelText("Command"));
    await user.type(screen.getByLabelText("URL or command"), "https://mcp.linear.app/mcp");
    expect(readDraft()).toMatchObject({
      transport: "http",
      url: "https://mcp.linear.app/mcp",
      command: "",
      name: "linear",
    });
    expect(screen.getByRole("radio", { name: "Remote server" })).toHaveAttribute(
      "aria-checked",
      "true"
    );

    await user.clear(screen.getByTestId("settings-mcp-servers-editor-name-input"));
    await user.type(screen.getByTestId("settings-mcp-servers-editor-name-input"), "mine");
    await user.type(screen.getByLabelText("Server URL"), "/v2");
    expect(readDraft().name).toBe("mine");
  });

  it("Should fill the form from pasted mcpServers JSON with one server", async () => {
    const user = userEvent.setup();
    render(<EditorHarness draft={emptyDraft("stdio")} />);

    await user.click(screen.getByRole("button", { name: "Paste JSON instead" }));
    fireEvent.change(screen.getByLabelText("Config from the docs"), {
      target: {
        value: JSON.stringify({
          mcpServers: {
            github: {
              command: "npx",
              args: ["-y", "@modelcontextprotocol/server-github"],
              env: { GITHUB_TOKEN: "" },
            },
          },
        }),
      },
    });

    expect(screen.getByTestId("settings-mcp-servers-editor-json-feedback")).toHaveTextContent(
      "Found 1 server in the JSON and filled in the rest."
    );
    expect(readDraft()).toMatchObject({
      name: "github",
      transport: "stdio",
      command: "npx",
      args: ["-y", "@modelcontextprotocol/server-github"],
    });
    const keys = screen.getByTestId("settings-mcp-editor-keys");
    expect(within(keys).getByText("1 required")).toBeInTheDocument();
    expect(within(keys).getByLabelText("GITHUB_TOKEN")).toHaveAttribute("type", "password");

    await user.type(within(keys).getByLabelText("GITHUB_TOKEN"), "ghp_x");
    expect(toRequest(readDraft()).secret_values).toEqual({ secret_env: { GITHUB_TOKEN: "ghp_x" } });
  });

  it("Should let the person pick one server when the JSON holds several", async () => {
    const user = userEvent.setup();
    render(<EditorHarness draft={emptyDraft("stdio")} />);

    await user.click(screen.getByRole("button", { name: "Paste JSON instead" }));
    fireEvent.change(screen.getByLabelText("Config from the docs"), {
      target: {
        value: JSON.stringify({
          mcpServers: {
            github: { command: "npx", args: ["gh"] },
            linear: { url: "https://mcp.linear.app/mcp" },
          },
        }),
      },
    });
    expect(screen.getByTestId("settings-mcp-servers-editor-json-feedback")).toHaveTextContent(
      "Found 2 servers. Pick the one to add."
    );
    expect(readDraft().command).toBe("");

    await user.click(screen.getByRole("radio", { name: "linear" }));
    expect(readDraft()).toMatchObject({
      name: "linear",
      transport: "http",
      url: "https://mcp.linear.app/mcp",
    });
    expect(screen.getByTestId("settings-mcp-servers-editor-json-feedback")).toHaveTextContent(
      "Found 2 servers and filled in linear."
    );
  });

  it("Should take back a pasted server once new JSON no longer names exactly one", async () => {
    const user = userEvent.setup();
    render(<EditorHarness draft={emptyDraft("stdio")} />);

    await user.click(screen.getByRole("button", { name: "Paste JSON instead" }));
    const input = screen.getByLabelText("Config from the docs");
    fireEvent.change(input, {
      target: { value: '{"mcpServers":{"github":{"command":"npx","args":["gh"]}}}' },
    });
    expect(readDraft()).toMatchObject({ name: "github", command: "npx", args: ["gh"] });

    // A second blob with two servers: nothing is picked yet, so nothing from the first stays.
    fireEvent.change(input, {
      target: {
        value:
          '{"mcpServers":{"linear":{"url":"https://mcp.linear.app/mcp"},"fs":{"command":"uvx"}}}',
      },
    });
    expect(screen.getByTestId("settings-mcp-servers-editor-json-feedback")).toHaveTextContent(
      "Found 2 servers. Pick the one to add."
    );
    expect(readDraft()).toMatchObject({ name: "", command: "", args: [], url: "" });
    expect(screen.getByRole("radio", { name: "linear" })).toHaveAttribute("aria-checked", "false");
  });

  it("Should explain JSON that can't be read and leave the draft alone", async () => {
    const user = userEvent.setup();
    render(<EditorHarness draft={emptyDraft("stdio")} />);

    await user.click(screen.getByRole("button", { name: "Paste JSON instead" }));
    fireEvent.change(screen.getByLabelText("Config from the docs"), {
      target: { value: "{ mcpServers" },
    });
    expect(screen.getByText("This isn't valid JSON yet.")).toBeInTheDocument();
    expect(readDraft()).toEqual(emptyDraft("stdio"));

    await user.click(screen.getByRole("button", { name: "Type a URL or command instead" }));
    expect(screen.getByLabelText("URL or command")).toBeInTheDocument();
  });

  it("Should hold Save while the open JSON supplies no picked server, keeping the earlier draft", async () => {
    const user = userEvent.setup();
    render(
      <ValidatingEditorHarness draft={{ ...emptyDraft("stdio"), name: "notes", command: "npx" }} />
    );
    const save = screen.getByTestId("settings-mcp-servers-editor-save");
    expect(save).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Paste JSON instead" }));
    expect(save).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Config from the docs"), {
      target: { value: "{ mcpServers" },
    });
    expect(save).toBeDisabled();
    // Leaving the JSON view keeps the earlier draft, which can save again.
    await user.click(screen.getByRole("button", { name: "Type a URL or command instead" }));
    expect(save).toBeEnabled();
    expect(screen.getByLabelText("Command")).toHaveValue("npx");

    await user.click(screen.getByRole("button", { name: "Paste JSON instead" }));
    fireEvent.change(screen.getByLabelText("Config from the docs"), {
      target: {
        value:
          '{"mcpServers":{"linear":{"url":"https://mcp.linear.app/mcp"},"fs":{"command":"uvx"}}}',
      },
    });
    expect(save).toBeDisabled();
    await user.click(screen.getByRole("radio", { name: "fs" }));
    expect(save).toBeEnabled();
  });

  // Invariant: a key row keeps its own state when an earlier row is removed.
  it("Should keep each key's state on its own row when an earlier key is removed", async () => {
    const user = userEvent.setup();
    const unset = (key: string) => ({
      key,
      binding: { mode: "typed" as const, existing: false, typedValue: "", vaultRef: "" },
    });
    render(
      <ValidatingEditorHarness
        draft={{
          ...emptyDraft("stdio"),
          name: "github",
          command: "bunx",
          secretEnv: [unset("FIRST_TOKEN"), unset("SECOND_TOKEN")],
        }}
      />
    );
    const message = "Enter a value or select a Vault reference";

    // Leaving the first key's value speaks up for that key alone.
    await user.click(screen.getByLabelText("FIRST_TOKEN"));
    await user.tab();
    expect(screen.getAllByText(message)).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(screen.getByTestId("settings-mcp-servers-editor-secret-remove-0"));

    // The second key moved up a row but was never left: it stays quiet.
    expect(screen.getByLabelText("SECOND_TOKEN")).toBeInTheDocument();
    expect(screen.queryByText(message)).toBeNull();
  });

  it("Should replace the launch configuration when the transport switches", async () => {
    const user = userEvent.setup();
    render(
      <EditorHarness
        draft={{
          ...emptyDraft("stdio"),
          name: "github",
          command: "bunx",
          args: ["@modelcontextprotocol/server-github"],
        }}
      />
    );

    await user.click(screen.getByTestId("settings-mcp-servers-editor-transport-remote"));

    const draft = readDraft();
    expect(draft.transport).toBe("http");
    expect(draft.command).toBe("");
    expect(draft.args).toEqual([]);
    // The request never carries the abandoned branch's fields.
    const request = toRequest({ ...draft, url: "https://mcp.example/mcp" });
    expect(request.server).not.toHaveProperty("command");
    expect(request.server).not.toHaveProperty("args");
  });

  // Invariant: a pristine form shows no field errors; a required field speaks up only after
  // it is blurred or edited, and a transport switch leaves the newly shown fields pristine.
  it("Should hold required-field errors until the field is touched", async () => {
    const user = userEvent.setup();
    render(<ValidatingEditorHarness draft={emptyDraft("stdio")} />);

    const name = screen.getByTestId("settings-mcp-servers-editor-name-input");
    const command = screen.getByTestId("settings-mcp-servers-editor-command-input");
    expect(screen.queryByText("Name is required")).toBeNull();
    expect(screen.queryByText("Command is required")).toBeNull();
    expect(name).not.toHaveAttribute("aria-invalid");
    expect(command).not.toHaveAttribute("aria-invalid");
    expect(screen.getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();

    await user.click(name);
    await user.click(command);
    expect(screen.getByText("Name is required")).toBeInTheDocument();
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(screen.queryByText("Command is required")).toBeNull();

    await user.click(screen.getByTestId("settings-mcp-servers-editor-transport-remote"));
    expect(screen.queryByText("URL is required for http transport")).toBeNull();
    const url = screen.getByTestId("settings-mcp-servers-editor-url-input");
    expect(url).not.toHaveAttribute("aria-invalid");

    await user.click(url);
    await user.click(name);
    expect(screen.getByText("URL is required for http transport")).toBeInTheDocument();
    expect(url).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();

    await user.click(screen.getByTestId("settings-mcp-servers-editor-transport-stdio"));
    expect(screen.queryByText("Command is required")).toBeNull();
    expect(screen.getByText("Name is required")).toBeInTheDocument();

    await user.type(screen.getByTestId("settings-mcp-servers-editor-command-input"), "x");
    await user.clear(screen.getByTestId("settings-mcp-servers-editor-command-input"));
    expect(screen.getByText("Command is required")).toBeInTheDocument();
  });

  it("Should keep remote HTTP transport fixed without an unsupported wire picker", () => {
    render(
      <EditorHarness draft={{ ...emptyDraft("http"), name: "linear", url: "https://x/mcp" }} />
    );

    expect(screen.queryByTestId("settings-mcp-servers-editor-remote-transport")).toBeNull();
    expect(readDraft().transport).toBe("http");
    const url = screen.getByTestId("settings-mcp-servers-editor-url-input");
    expect(url).toHaveValue("https://x/mcp");
  });

  it("Should lock the name on edit and keep the scope a read-only fact", () => {
    render(
      <MCPServerEditor
        {...makeProps({
          mode: "edit",
          draft: { ...emptyDraft("stdio"), name: "github", command: "bunx" },
          scopeChoice: { value: "user", projectName: "compozy", disabled: true },
        })}
      />
    );

    expect(screen.getByRole("dialog", { name: "Edit github" })).toBeInTheDocument();
    expect(screen.getByTestId("settings-mcp-servers-editor-name-input")).toBeDisabled();
    expect(screen.getByRole("radio", { name: "All projects" })).toBeDisabled();
    expect(screen.getByRole("radio", { name: "Only compozy" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Paste JSON instead" })).toBeNull();
  });

  it("Should let a new server pick all projects or only the open project", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <MCPServerEditor
        {...makeProps({ scopeChoice: { value: "user", projectName: "compozy", onChange } })}
      />
    );

    await user.click(screen.getByRole("radio", { name: "Only compozy" }));
    expect(onChange).toHaveBeenCalledWith("workspace");
  });

  it("Should keep the name editable while creating", () => {
    render(<MCPServerEditor {...makeProps()} />);

    expect(screen.getByTestId("settings-mcp-servers-editor-name-input")).toBeEnabled();
    expect(screen.queryByTestId("settings-mcp-servers-editor-scope")).toBeNull();
  });

  it("Should gate the primary on validity and retain the draft on a save error", () => {
    render(
      <MCPServerEditor
        {...makeProps({ isValid: false, saveError: "settings write rejected the payload" })}
      />
    );

    expect(screen.getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();
    expect(screen.getByTestId("settings-mcp-servers-editor-error")).toHaveTextContent(
      "settings write rejected the payload"
    );
    expect(screen.getByTestId("settings-mcp-servers-editor-command-input")).toHaveValue("bunx");
  });

  it("Should expose Remove only on an existing server", () => {
    const onRemove = vi.fn();
    const { rerender } = render(<MCPServerEditor {...makeProps({ onRemove })} />);

    expect(screen.queryByTestId("settings-mcp-servers-editor-remove")).toBeNull();

    rerender(<MCPServerEditor {...makeProps({ mode: "edit", onRemove })} />);

    expect(screen.getByTestId("settings-mcp-servers-editor-remove")).toBeInTheDocument();
  });
});
