import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { emptyDraft, toRequest, type MCPDraft } from "../../lib/mcp-editor-model";
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

function readDraft(): MCPDraft {
  return JSON.parse(screen.getByTestId("mcp-draft-probe").textContent ?? "{}");
}

describe("MCPServerEditor", () => {
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

  it("Should offer remote authentication only for a remote transport", async () => {
    const user = userEvent.setup();
    render(<EditorHarness />);

    await user.click(screen.getByTestId("settings-mcp-servers-editor-mode-advanced"));
    expect(screen.queryByTestId("settings-mcp-editor-oauth")).toBeNull();

    await user.click(screen.getByTestId("settings-mcp-servers-editor-transport-remote"));

    expect(screen.getByTestId("settings-mcp-editor-oauth")).toBeInTheDocument();
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

  it("Should let a remote server pick its wire transport without leaving the remote branch", async () => {
    const user = userEvent.setup();
    render(
      <EditorHarness draft={{ ...emptyDraft("http"), name: "linear", url: "https://x/mcp" }} />
    );

    await user.selectOptions(
      screen.getByTestId("settings-mcp-servers-editor-remote-transport"),
      "sse"
    );

    expect(readDraft().transport).toBe("sse");
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
