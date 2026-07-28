import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { emptyDraft } from "../../lib/mcp-editor-model";
import { MCPEditorOAuthSection } from "../mcp-editor-oauth-section";
import { MCPEditorProcessSection } from "../mcp-editor-stdio-section";
import { MCPSecretBindingControl } from "../mcp-secret-binding";

const SELECTABLE_VAULT_REF = "vault:mcp/ws/ws-platform/shared/QA_TYPED_TOKEN";

describe("MCP editor configured secret presence", () => {
  it("Should offer preservation for a configured stdio secret without rendering a binding", () => {
    render(
      <MCPEditorProcessSection
        draft={{
          ...emptyDraft("stdio"),
          secretEnv: [
            {
              key: "QA_TYPED_TOKEN",
              binding: { mode: "preserve", existing: true, typedValue: "", vaultRef: "" },
            },
          ],
        }}
        errors={{}}
        vaultInventory={{ status: "ready", refs: [SELECTABLE_VAULT_REF] }}
        onChange={() => undefined}
      />
    );

    expect(screen.getByRole("radio", { name: /Keep configured/i })).toHaveAttribute(
      "aria-checked",
      "true"
    );
    expect(screen.getByText("Configured")).toBeInTheDocument();
    expect(screen.queryByText(SELECTABLE_VAULT_REF)).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Existing Vault ref")).not.toBeInTheDocument();
  });

  it("Should disclose only an explicitly selected Vault inventory ref", () => {
    let selected = emptyDraft("http").oauth.clientSecret;
    const { rerender } = render(
      <MCPEditorOAuthSection
        oauth={{
          ...emptyDraft("http").oauth,
          enabled: true,
          clientSecret: { mode: "preserve", existing: true, typedValue: "", vaultRef: "" },
        }}
        vaultInventory={{ status: "ready", refs: [SELECTABLE_VAULT_REF] }}
        errors={{}}
        onChange={oauth => {
          selected = oauth.clientSecret;
        }}
      />
    );

    expect(screen.getByText("Configured")).toBeInTheDocument();
    expect(screen.queryByText(SELECTABLE_VAULT_REF)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: /Use Vault/i }));
    rerender(
      <MCPEditorOAuthSection
        oauth={{ ...emptyDraft("http").oauth, enabled: true, clientSecret: selected }}
        vaultInventory={{ status: "ready", refs: [SELECTABLE_VAULT_REF] }}
        errors={{}}
        onChange={() => undefined}
      />
    );

    expect(screen.getByLabelText("Existing Vault ref")).toHaveValue(SELECTABLE_VAULT_REF);
  });

  it("Should surface an empty configured OAuth secret replacement", () => {
    render(
      <MCPEditorOAuthSection
        oauth={{
          ...emptyDraft("http").oauth,
          enabled: true,
          clientSecret: { mode: "typed", existing: true, typedValue: "", vaultRef: "" },
        }}
        vaultInventory={{ status: "ready", refs: [] }}
        errors={{ clientSecret: "Enter a replacement value or select a Vault reference" }}
        onChange={() => undefined}
      />
    );

    expect(screen.getByText("Enter a replacement value or select a Vault reference")).toBeVisible();
  });
});

describe("MCP editor stdio secret binding errors", () => {
  it("Should surface a per-row binding error and mark the key input invalid", () => {
    render(
      <MCPEditorProcessSection
        draft={{
          ...emptyDraft("stdio"),
          secretEnv: [
            {
              key: "TOKEN",
              binding: { mode: "typed", existing: false, typedValue: "", vaultRef: "" },
            },
          ],
        }}
        errors={{ secretEnv: { 0: "Enter a value or select a Vault reference" } }}
        vaultInventory={{ status: "ready", refs: [] }}
        onChange={() => undefined}
      />
    );

    expect(screen.getByTestId("settings-mcp-servers-editor-secret-error-0")).toHaveTextContent(
      "Enter a value or select a Vault reference"
    );
    expect(screen.getByTestId("settings-mcp-servers-editor-secret-key-0")).toHaveAttribute(
      "aria-invalid",
      "true"
    );
  });
});

describe("MCP editor Vault inventory states", () => {
  const refBinding = { mode: "ref" as const, existing: false, typedValue: "", vaultRef: "" };

  it("Should distinguish loading and error from a confirmed empty inventory", () => {
    const retry = vi.fn();
    const { rerender } = render(
      <MCPSecretBindingControl
        binding={refBinding}
        vaultInventory={{ status: "loading" }}
        idPrefix="inventory"
        onChange={() => undefined}
      />
    );

    expect(screen.getByTestId("inventory-ref-loading")).toBeVisible();
    expect(screen.queryByTestId("inventory-ref-empty")).not.toBeInTheDocument();

    rerender(
      <MCPSecretBindingControl
        binding={refBinding}
        vaultInventory={{ status: "error", message: "Vault unavailable", retry }}
        idPrefix="inventory"
        onChange={() => undefined}
      />
    );
    expect(screen.getByTestId("inventory-ref-error")).toHaveTextContent("Vault unavailable");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(screen.queryByTestId("inventory-ref-empty")).not.toBeInTheDocument();

    rerender(
      <MCPSecretBindingControl
        binding={refBinding}
        vaultInventory={{ status: "ready", refs: [] }}
        idPrefix="inventory"
        onChange={() => undefined}
      />
    );
    expect(screen.getByTestId("inventory-ref-empty")).toBeVisible();
  });

  it("Should write the selected ref when an asynchronously loaded inventory enables Vault", () => {
    const onChange = vi.fn();
    const binding = { mode: "typed" as const, existing: false, typedValue: "", vaultRef: "" };
    const { rerender } = render(
      <MCPSecretBindingControl
        binding={binding}
        vaultInventory={{ status: "ready", refs: [] }}
        idPrefix="async-inventory"
        onChange={onChange}
      />
    );
    expect(screen.getByRole("radio", { name: /Use Vault/i })).toBeDisabled();

    rerender(
      <MCPSecretBindingControl
        binding={binding}
        vaultInventory={{ status: "ready", refs: [SELECTABLE_VAULT_REF] }}
        idPrefix="async-inventory"
        onChange={onChange}
      />
    );
    fireEvent.click(screen.getByRole("radio", { name: /Use Vault/i }));
    expect(onChange).toHaveBeenCalledWith({
      ...binding,
      mode: "ref",
      vaultRef: SELECTABLE_VAULT_REF,
    });
  });
});
