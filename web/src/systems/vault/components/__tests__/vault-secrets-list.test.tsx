import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { VaultSecret } from "../../types";
import { VaultSecretSheet } from "../vault-secret-sheet";
import { VaultSecretsList } from "../vault-secrets-list";

const secrets: VaultSecret[] = [
  {
    ref: "vault:sessions/session_01/api_key",
    namespace: "sessions",
    kind: "api_key",
    present: true,
    created_at: "2026-04-17T18:00:00Z",
    updated_at: "2026-04-17T18:14:00Z",
  },
  {
    ref: "vault:providers/codex/api_key",
    namespace: "providers",
    kind: "",
    present: true,
    created_at: "2026-04-17T18:00:00Z",
    updated_at: "not-a-date",
  },
];

describe("VaultSecretsList", () => {
  it("Should render ready rows with namespace tones", () => {
    render(<VaultSecretsList secrets={secrets} />);

    expect(screen.getByTestId("vault-secrets-list")).toHaveAttribute(
      "data-slot",
      "data-surface-content"
    );
    expect(screen.getByText("sessions")).toHaveAttribute("data-tone", "info");
    expect(screen.getByText("providers")).toHaveAttribute("data-tone", "neutral");
    expect(screen.getAllByTestId("vault-secrets-row")).toHaveLength(2);
  });

  it("Should render the secret kind as a neutral pill (— no tone until enum lands)", () => {
    render(<VaultSecretsList secrets={secrets} />);
    const kindPill = screen.getByTestId(`vault-secrets-kind-${secrets[0].ref}`);
    expect(kindPill).toHaveAttribute("data-tone", "neutral");
    expect(kindPill).toHaveTextContent("api_key");
    // Empty kind falls back to "--" instead of a pill so absent kinds don't render colour.
    expect(screen.getByTestId(`vault-secrets-kind-empty-${secrets[1].ref}`)).toHaveTextContent(
      "--"
    );
  });

  it("Should render updated timestamps via <Time>", () => {
    render(<VaultSecretsList secrets={secrets} />);
    const time = screen.getByTestId(`vault-secrets-updated-${secrets[0].ref}`);
    expect(time.tagName.toLowerCase()).toBe("time");
    expect(time.getAttribute("datetime")).toBe(secrets[0].updated_at);
  });

  it("Should route loading, error, and empty through DataSurface slots", () => {
    const { rerender } = render(<VaultSecretsList secrets={[]} isLoading />);
    expect(screen.getByTestId("vault-secrets-list-loading")).toHaveAttribute(
      "data-slot",
      "block-loading"
    );

    rerender(<VaultSecretsList secrets={[]} error={new Error("vault down")} />);
    expect(screen.getByTestId("vault-secrets-list-error")).toHaveTextContent("vault down");

    rerender(<VaultSecretsList secrets={[]} emptyTitle="No provider secrets" />);
    expect(screen.getByTestId("vault-secrets-list-empty")).toHaveTextContent("No provider secrets");
  });

  it("Should call onDelete for the selected secret", () => {
    const onDelete = vi.fn();
    render(<VaultSecretsList secrets={secrets} onDelete={onDelete} />);

    fireEvent.click(screen.getByTestId("vault-secrets-delete-vault:providers/codex/api_key"));
    expect(onDelete).toHaveBeenCalledWith(secrets[1]);
  });

  it("Should call onSelect when an interactive row is activated", () => {
    const onSelect = vi.fn();
    render(<VaultSecretsList onSelect={onSelect} secrets={secrets} />);

    fireEvent.click(screen.getByTestId(`vault-secrets-select-${secrets[0].ref}`));
    expect(onSelect).toHaveBeenCalledWith(secrets[0]);
  });

  it("Should render the cards grid when view is cards", () => {
    const onSelect = vi.fn();
    render(<VaultSecretsList onSelect={onSelect} secrets={secrets} view="cards" />);

    expect(screen.getByTestId("vault-secrets-list-card-grid")).toBeInTheDocument();
    expect(screen.getAllByTestId("vault-secrets-card")).toHaveLength(2);
    fireEvent.click(screen.getByTestId(`vault-secrets-select-${secrets[1].ref}`));
    expect(onSelect).toHaveBeenCalledWith(secrets[1]);
  });

  it("Should call onDelete from the cards grid", () => {
    const onDelete = vi.fn();
    render(<VaultSecretsList onDelete={onDelete} secrets={secrets} view="cards" />);

    fireEvent.click(screen.getByTestId(`vault-secrets-delete-${secrets[0].ref}`));

    expect(onDelete).toHaveBeenCalledWith(secrets[0]);
  });

  it("Should mark the selected row", () => {
    render(<VaultSecretsList onSelect={vi.fn()} secrets={secrets} selectedRef={secrets[0].ref} />);

    expect(screen.getAllByTestId("vault-secrets-row")[0]).toHaveAttribute("data-selected", "true");
  });
});

describe("VaultSecretSheet", () => {
  const secret = secrets[0];

  it("Should render redacted metadata tiles without plaintext values", () => {
    const { container } = render(
      <VaultSecretSheet
        deleteIsDisabled={false}
        onOpenChange={vi.fn()}
        onReplace={vi.fn()}
        onReplaceValueChange={vi.fn()}
        onRequestDelete={vi.fn()}
        open
        replaceError={null}
        replaceIsPending={false}
        replaceIsValid={false}
        replaceValue=""
        secret={secret}
      />
    );

    expect(screen.getByTestId("vault-secret-sheet-title")).toHaveTextContent("api_key");
    expect(screen.getByTestId("vault-secret-sheet-ref")).toHaveTextContent(secret.ref);
    expect(screen.getByTestId("vault-secret-sheet-value")).toHaveTextContent("write-only");
    expect(screen.getByTestId("vault-secret-sheet-foot")).toHaveTextContent(
      `agh vault put ${secret.ref} --value-stdin`
    );
    expect(container.textContent).not.toContain("plaintext-secret");
  });

  it("Should forward replace and delete actions", () => {
    const onReplace = vi.fn();
    const onReplaceValueChange = vi.fn();
    const onRequestDelete = vi.fn();

    render(
      <VaultSecretSheet
        deleteIsDisabled={false}
        onOpenChange={vi.fn()}
        onReplace={onReplace}
        onReplaceValueChange={onReplaceValueChange}
        onRequestDelete={onRequestDelete}
        open
        replaceError={null}
        replaceIsPending={false}
        replaceIsValid
        replaceValue="rotated"
        secret={secret}
      />
    );

    fireEvent.change(screen.getByTestId("vault-secret-sheet-replace-input"), {
      target: { value: "next-value" },
    });
    expect(onReplaceValueChange).toHaveBeenCalledWith("next-value");

    fireEvent.click(screen.getByTestId("vault-secret-sheet-replace-save"));
    expect(onReplace).toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("vault-secret-sheet-delete"));
    expect(onRequestDelete).toHaveBeenCalledWith(secret);
  });

  it("Should handle clipboard rejection and report that the reference was not copied", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("Clipboard blocked"));
    vi.stubGlobal("navigator", { clipboard: { writeText } });

    render(
      <VaultSecretSheet
        deleteIsDisabled={false}
        onOpenChange={vi.fn()}
        onReplace={vi.fn()}
        onReplaceValueChange={vi.fn()}
        onRequestDelete={vi.fn()}
        open
        replaceError={null}
        replaceIsPending={false}
        replaceIsValid={false}
        replaceValue=""
        secret={secret}
      />
    );

    fireEvent.click(screen.getByTestId("vault-secret-sheet-copy"));

    await waitFor(() =>
      expect(screen.getByTestId("vault-secret-sheet-copy-error")).toHaveTextContent(
        "Vault reference could not be copied."
      )
    );
    expect(writeText).toHaveBeenCalledWith(secret.ref);

    vi.unstubAllGlobals();
  });
});
