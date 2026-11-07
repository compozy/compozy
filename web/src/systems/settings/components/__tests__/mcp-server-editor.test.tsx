import { render, screen } from "@testing-library/react";
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

    expect(screen.getByRole("dialog", { name: "Add MCP server" })).toBeInTheDocument();
    expect(screen.getByTestId("settings-mcp-servers-editor-save")).toHaveTextContent("Add server");
    expect(screen.getByTestId("settings-mcp-servers-editor-cancel")).toBeInTheDocument();
  });

  it("Should keep the connection in Simple and the process environment in Advanced", async () => {
    const user = userEvent.setup();
    render(<EditorHarness />);

    expect(screen.getByTestId("settings-mcp-servers-editor-command-input")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-mcp-editor-stdio")).toBeNull();

    await user.click(screen.getByTestId("settings-mcp-servers-editor-mode-advanced"));

    expect(screen.getByTestId("settings-mcp-editor-stdio")).toBeInTheDocument();
    // Advanced never hides the required connection fields.
    expect(screen.getByTestId("settings-mcp-servers-editor-command-input")).toBeInTheDocument();
  });

  it("Should explain daemon-owned authorization only for a remote transport", async () => {
    const user = userEvent.setup();
    render(<EditorHarness />);

    await user.click(screen.getByTestId("settings-mcp-servers-editor-mode-advanced"));
    expect(screen.queryByTestId("settings-mcp-editor-oauth")).toBeNull();

    await user.click(screen.getByTestId("settings-mcp-servers-editor-transport-remote"));

    expect(screen.getByTestId("settings-mcp-editor-oauth")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "About authorization" })).toBeInTheDocument();
    expect(screen.queryByTestId("settings-mcp-editor-stdio")).toBeNull();
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

  it("Should lock name and scope on edit as readable identity, never as inputs", () => {
    render(
      <MCPServerEditor
        {...makeProps({ mode: "edit", draft: { ...emptyDraft("stdio"), name: "github" } })}
      />
    );

    const rows = document.body.querySelector('[data-slot="immutable-identity-rows"]');
    expect(rows).not.toBeNull();
    expect(rows).toHaveTextContent("github");
    expect(rows).toHaveTextContent("workspace");
    expect((rows as HTMLElement).querySelectorAll("input")).toHaveLength(0);
    expect(screen.queryByTestId("settings-mcp-servers-editor-name-input")).toBeNull();
  });

  it("Should keep the name editable while creating", () => {
    render(<MCPServerEditor {...makeProps()} />);

    expect(screen.getByTestId("settings-mcp-servers-editor-name-input")).toBeInTheDocument();
    expect(document.body.querySelector('[data-slot="immutable-identity-rows"]')).toBeNull();
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
