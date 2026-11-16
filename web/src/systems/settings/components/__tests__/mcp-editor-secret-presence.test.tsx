import { fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { emptyDraft, type MCPSecretEnvEntry } from "../../lib/mcp-editor-model";
import { MCPEditorProcessSection } from "../mcp-editor-stdio-section";
import { MCPSecretKeyField, type MCPVaultInventory } from "../mcp-secret-binding";

const SELECTABLE_VAULT_REF = "vault:mcp/ws/ws-platform/shared/QA_TYPED_TOKEN";

function configured(key = "QA_TYPED_TOKEN"): MCPSecretEnvEntry {
  return {
    key,
    originalKey: key,
    binding: { mode: "preserve", existing: true, typedValue: "", vaultRef: "" },
  };
}

/** Stateful host so Replace, Cancel and mode switches behave like the editor. */
function KeyHarness({
  initial,
  vaultInventory = { status: "ready", refs: [] },
  onEntry = () => undefined,
}: {
  initial: MCPSecretEnvEntry;
  vaultInventory?: MCPVaultInventory;
  onEntry?: (entry: MCPSecretEnvEntry) => void;
}) {
  const [entry, setEntry] = useState(initial);
  return (
    <MCPSecretKeyField
      entry={entry}
      index={0}
      onChange={next => {
        setEntry(next);
        onEntry(next);
      }}
      vaultInventory={vaultInventory}
    />
  );
}

// Invariant: a configured key is reported by presence only — never its value or bound ref —
// and is kept unless the person explicitly replaces it.
describe("MCP editor configured secret presence", () => {
  it("Should keep a configured key without rendering its binding until it is replaced", () => {
    const onEntry = vi.fn();
    render(
      <KeyHarness
        initial={configured()}
        onEntry={onEntry}
        vaultInventory={{ status: "ready", refs: [SELECTABLE_VAULT_REF] }}
      />
    );

    expect(
      screen.getByTestId("settings-mcp-servers-editor-secret-value-0-presence")
    ).toHaveTextContent("Saved in your vault");
    expect(screen.queryByText(SELECTABLE_VAULT_REF)).not.toBeInTheDocument();
    expect(screen.queryByLabelText("QA_TYPED_TOKEN")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("settings-mcp-servers-editor-secret-value-0-replace"));
    expect(onEntry).toHaveBeenLastCalledWith(
      expect.objectContaining({ binding: expect.objectContaining({ mode: "typed" }) })
    );
    fireEvent.change(screen.getByLabelText("QA_TYPED_TOKEN"), { target: { value: "next" } });

    fireEvent.click(screen.getByTestId("settings-mcp-servers-editor-secret-value-0-cancel"));
    expect(onEntry).toHaveBeenLastCalledWith(
      expect.objectContaining({
        binding: expect.objectContaining({ mode: "preserve", typedValue: "" }),
      })
    );
  });

  it("Should ask for a new value once a configured key is renamed", () => {
    render(<KeyHarness initial={{ ...configured(), key: "RENAMED_TOKEN" }} />);

    expect(
      screen.queryByTestId("settings-mcp-servers-editor-secret-value-0-presence")
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("RENAMED_TOKEN")).toHaveAttribute("type", "password");
  });
});

describe("MCP editor stdio secret binding errors", () => {
  it("Should mark the key name invalid under More options when its binding has no value", () => {
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
        onChange={() => undefined}
        secretRowKeys={{ keys: ["secret-0"], append: () => undefined, remove: () => undefined }}
      />
    );

    expect(screen.getByTestId("settings-mcp-servers-editor-secret-key-0")).toHaveAttribute(
      "aria-invalid",
      "true"
    );
  });
});

describe("MCP editor Vault inventory states", () => {
  const refEntry: MCPSecretEnvEntry = {
    key: "TOKEN",
    binding: { mode: "ref", existing: false, typedValue: "", vaultRef: "" },
  };

  it("Should distinguish loading and error from a confirmed empty inventory", () => {
    const retry = vi.fn();
    const field = (vaultInventory: MCPVaultInventory) => (
      <MCPSecretKeyField
        entry={refEntry}
        index={0}
        onChange={() => undefined}
        vaultInventory={vaultInventory}
      />
    );
    const { rerender } = render(field({ status: "loading" }));
    const sources = () => screen.getByTestId("settings-mcp-servers-editor-secret-value-0-sources");

    expect(within(sources()).getByRole("status")).toBeVisible();
    expect(screen.queryByText("No keys saved in your vault for MCP servers yet.")).toBeNull();

    rerender(field({ status: "error", message: "Vault unavailable", retry }));
    expect(sources()).toHaveTextContent("Vault unavailable");
    fireEvent.click(within(sources()).getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(screen.queryByText("No keys saved in your vault for MCP servers yet.")).toBeNull();

    rerender(field({ status: "ready", refs: [] }));
    expect(screen.getByText("No keys saved in your vault for MCP servers yet.")).toBeVisible();
  });

  it("Should bind the first ref when an asynchronously loaded inventory enables Vault", () => {
    const onEntry = vi.fn();
    const typed: MCPSecretEnvEntry = {
      key: "TOKEN",
      binding: { mode: "typed", existing: false, typedValue: "", vaultRef: "" },
    };
    const { rerender } = render(
      <KeyHarness initial={typed} onEntry={onEntry} vaultInventory={{ status: "loading" }} />
    );

    rerender(
      <KeyHarness
        initial={typed}
        onEntry={onEntry}
        vaultInventory={{ status: "ready", refs: [SELECTABLE_VAULT_REF] }}
      />
    );
    fireEvent.click(screen.getByTestId("settings-mcp-servers-editor-secret-value-0-mode-source"));
    expect(onEntry).toHaveBeenLastCalledWith({
      ...typed,
      binding: { ...typed.binding, mode: "ref", vaultRef: SELECTABLE_VAULT_REF },
    });
  });
});
