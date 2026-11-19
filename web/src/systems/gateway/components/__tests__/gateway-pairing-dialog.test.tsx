// Suite: pairing issuance
// Invariant: the one-time code is obtainable without a camera — the six-digit manual code is the
// primary path, grouped for reading, announced digit by digit, and copied as exactly those digits.
// Every scannable code keeps an equally usable copyable text path, and the scannable link keeps the
// secret in its fragment so no server-visible surface ever sees it.
// Owning layer: the gateway pairing issuance component.
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { GatewayPairingDialog } from "../gateway-pairing-dialog";

const ARTIFACT = {
  artifact: "cpz_gwp_2f8c1d5a9b",
  expires_at: "2099-01-01T00:00:00Z",
  manual_code: "482913",
};

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

function stubClipboard(writeText: (value: string) => Promise<void>): ReturnType<typeof vi.fn> {
  const spy = vi.fn(writeText);
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: spy },
    configurable: true,
  });
  return spy;
}

describe("GatewayPairingDialog", () => {
  afterEach(() => {
    Reflect.deleteProperty(navigator, "clipboard");
  });

  it("Should describe the code as single-use without claiming the visible value disappeared", () => {
    renderDialog();

    expect(screen.getByText(/works once/i)).toBeInTheDocument();
    expect(screen.getByText(/expires or is used/i)).toBeInTheDocument();
    expect(screen.queryByText(/cannot be shown again/i)).not.toBeInTheDocument();
  });

  it("Should lead with the six-digit manual code, grouped for reading and spoken digit by digit", () => {
    renderDialog();

    expect(screen.getByText("482 913")).toBeInTheDocument();
    expect(screen.getByText("4 8 2 9 1 3")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /copy code/i })).toBeInTheDocument();
  });

  it("Should copy exactly the six digits and confirm the copy to assistive technology", async () => {
    const writeText = stubClipboard(() => Promise.resolve());
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: /copy code/i }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith("482913"));
    expect(await screen.findByText("Pairing code copied")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /copied/i })).toBeInTheDocument();
  });

  it("Should report a failed copy rather than confirming one that never reached the clipboard", async () => {
    stubClipboard(() => Promise.reject(new Error("denied")));
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: /copy code/i }));

    expect(
      await screen.findByText("Couldn't copy the pairing code. Select it and copy it by hand.")
    ).toBeInTheDocument();
    expect(screen.getByTestId("gateway-pairing-manual-code-error")).toHaveTextContent(
      /copy it by hand/i
    );
    expect(screen.queryByText("Pairing code copied")).not.toBeInTheDocument();
  });

  it("Should show a failed link copy beside the control, not only in the live region", async () => {
    stubClipboard(() => Promise.reject(new Error("denied")));
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: /copy link/i }));

    expect(await screen.findByTestId("gateway-pairing-link-error")).toHaveTextContent(
      /copy it by hand/i
    );
  });

  it("Should offer the QR code, the link, and the long code as copyable secondary paths", () => {
    renderDialog();

    expect(screen.getByRole("img", { name: /scan/i })).toBeInTheDocument();
    expect(screen.getByTestId("gateway-pairing-artifact-value")).toHaveTextContent(
      ARTIFACT.artifact
    );
    expect(screen.getByRole("button", { name: /copy link/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /copy long code/i })).toBeInTheDocument();
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
    expect(screen.getByText("482 913")).toBeInTheDocument();
    expect(screen.getByTestId("gateway-pairing-artifact-value")).toHaveTextContent(
      ARTIFACT.artifact
    );

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

    expect(mint).toHaveTextContent("Create a new code");
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
