import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { VaultSecret } from "../../types";
import { SessionVaultPanel } from "../session-vault-panel";

const secrets: VaultSecret[] = [
  {
    ref: "vault:sessions/session_01/api_key",
    namespace: "sessions",
    kind: "api_key",
    present: true,
    created_at: "2026-04-17T18:00:00Z",
    updated_at: "2026-04-17T18:14:00Z",
  },
];

describe("SessionVaultPanel", () => {
  it("Should render session vault rows with 'Vault secret' title + mono ref subtitle", () => {
    render(<SessionVaultPanel secrets={secrets} sessionId="session_01" />);

    const list = screen.getByTestId("session-inspector-vault-list");
    const row = screen.getByTestId("session-inspector-vault-row");
    expect(list).toHaveAttribute("data-slot", "item-group");
    expect(row).toHaveAttribute("data-slot", "item");
    expect(screen.getByTestId("session-inspector-vault-title")).toHaveTextContent("Vault secret");
    const ref = screen.getByTestId("session-inspector-vault-ref");
    expect(ref).toHaveTextContent("api_key");
    expect(ref.className).toMatch(/font-mono/);
    expect(ref.className).toMatch(/\btext-faint\b/);
  });

  it("Should render the row's updated timestamp via <Time>", () => {
    render(<SessionVaultPanel secrets={secrets} sessionId="session_01" />);
    const updated = screen.getByTestId("session-inspector-vault-updated");
    expect(updated.tagName.toLowerCase()).toBe("time");
    expect(updated.getAttribute("datetime")).toBe(secrets[0].updated_at);
  });

  it("Should render loading, empty, and error states through DataSurface slots", () => {
    const { rerender } = render(<SessionVaultPanel secrets={[]} isLoading />);
    expect(screen.getByTestId("session-inspector-vault-loading")).toHaveAttribute(
      "data-slot",
      "block-loading"
    );

    rerender(<SessionVaultPanel secrets={[]} />);
    expect(screen.getByTestId("session-inspector-vault-empty")).toHaveTextContent(
      "No session vault secrets"
    );

    rerender(<SessionVaultPanel secrets={[]} error={new Error("vault down")} />);
    expect(screen.getByTestId("session-inspector-vault-error")).toHaveTextContent("vault down");
  });
});
