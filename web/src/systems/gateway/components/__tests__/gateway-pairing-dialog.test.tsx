// Suite: pairing issuance
// Invariant: the one-time artifact is obtainable without a camera — every scannable code has an
// equally usable copyable text path — and the scannable link keeps the secret in its fragment so no
// server-visible surface ever sees it.
// Owning layer: the gateway pairing issuance component.
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { GatewayPairingDialog } from "../gateway-pairing-dialog";

const ARTIFACT = { artifact: "cpz_gwp_2f8c1d5a9b", expires_at: "2099-01-01T00:00:00Z" };

function renderDialog(overrides: Partial<React.ComponentProps<typeof GatewayPairingDialog>> = {}) {
  const props = {
    open: true,
    isMinting: false,
    artifact: ARTIFACT,
    pairingAddress: "https://private.gateway.test",
    onOpenChange: vi.fn(),
    onMint: vi.fn(),
    ...overrides,
  };
  render(<GatewayPairingDialog {...props} />);
  return props;
}

describe("GatewayPairingDialog", () => {
  it("Should offer the artifact as copyable text beside the scannable code", () => {
    renderDialog();

    expect(screen.getByRole("img", { name: /scan/i })).toBeInTheDocument();
    expect(screen.getByTestId("gateway-pairing-code")).toHaveTextContent(ARTIFACT.artifact);
    expect(screen.getByRole("button", { name: /copy pairing code/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /copy pairing link/i })).toBeInTheDocument();
  });

  it("Should keep the artifact out of the scannable link's server-visible part", () => {
    renderDialog();
    const link = screen.getByTestId("gateway-pairing-link").textContent ?? "";

    expect(link).toContain("https://private.gateway.test");
    const requestLine = link.slice(0, link.indexOf("#"));
    expect(requestLine).not.toContain(ARTIFACT.artifact);
    expect(link).toContain(`#pair=${ARTIFACT.artifact}`);
  });

  it("Should tell the operator how to recover when no verified address is published yet", () => {
    renderDialog({ pairingAddress: undefined });

    expect(screen.queryByRole("img", { name: /scan/i })).not.toBeInTheDocument();
    expect(screen.getByTestId("gateway-pairing-code")).toHaveTextContent(ARTIFACT.artifact);

    const recovery = screen.getByTestId("gateway-pairing-no-address");
    expect(recovery).toHaveTextContent("Turn on the private overlay");
    expect(recovery).toHaveTextContent("wait for its address to be verified");
    expect(recovery).toHaveTextContent("mint a new code");
    expect(recovery).toHaveTextContent("may expire while you wait");
  });

  it("Should show no artifact before one has been minted", () => {
    renderDialog({ artifact: null });

    expect(screen.queryByTestId("gateway-pairing-artifact")).not.toBeInTheDocument();
    expect(screen.getByText("No pairing code yet")).toBeInTheDocument();
  });

  it("Should offer a re-mint once a code exists, because a spent or expired one is not revivable", () => {
    const { onMint } = renderDialog();
    const mint = screen.getByTestId("gateway-pairing-mint");

    expect(mint).toHaveTextContent("Mint a new code");
    mint.click();
    expect(onMint).toHaveBeenCalledTimes(1);
  });

  it("Should report a refused mint instead of rendering a stale code", () => {
    renderDialog({ artifact: null, error: "gateway pairing limit reached" });

    expect(screen.getByTestId("gateway-pairing-error")).toHaveTextContent(
      "gateway pairing limit reached"
    );
    expect(screen.queryByTestId("gateway-pairing-artifact")).not.toBeInTheDocument();
  });
});
