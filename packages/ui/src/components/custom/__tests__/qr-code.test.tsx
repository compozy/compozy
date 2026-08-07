import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { encode } from "uqr";

import { QrCode } from "../qr-code";

// Pairing links carry their artifact in the fragment, which is what the product
// actually encodes into a QR code.
const PAYLOAD = "https://gateway.example.test/#pair=cpz_gwp_2f8c1d5a9b";

describe("QrCode", () => {
  it("Should expose the matrix as a labelled image rather than raw payload text", () => {
    render(<QrCode value={PAYLOAD} label="Scan to pair this device" />);
    const image = screen.getByRole("img", { name: "Scan to pair this device" });
    expect(image).toBeInTheDocument();
    expect(image.textContent).not.toContain(PAYLOAD);
  });

  it("Should size the viewBox to the encoded matrix including its quiet zone", () => {
    const quietZone = 4;
    const expected = encode(PAYLOAD, { border: quietZone, ecc: "M" });
    const { container } = render(
      <QrCode value={PAYLOAD} label="Scan to pair" quietZone={quietZone} />
    );
    const svg = container.querySelector<SVGSVGElement>('[data-slot="qr-code"]');
    expect(svg?.getAttribute("viewBox")).toBe(`0 0 ${expected.size} ${expected.size}`);
  });

  it("Should re-encode when the payload changes so a re-minted artifact is never stale", () => {
    const { container, rerender } = render(<QrCode value={PAYLOAD} label="Scan to pair" />);
    const first = container.querySelector('[data-slot="qr-code"] path')?.getAttribute("d");
    rerender(<QrCode value={`${PAYLOAD}-rotated`} label="Scan to pair" />);
    const second = container.querySelector('[data-slot="qr-code"] path')?.getAttribute("d");
    expect(second).not.toBe(first);
  });

  it("Should keep dark modules on a light plate so scanners are not handed an inverted code", () => {
    const { container } = render(<QrCode value={PAYLOAD} label="Scan to pair" />);
    const plate = container.querySelector('[data-slot="qr-code"] rect');
    const path = container.querySelector('[data-slot="qr-code"] path');
    expect(plate).toHaveClass("fill-viz-cell");
    expect(path).toHaveClass("fill-accent-ink");
  });

  it("Should apply the requested rendered size", () => {
    const { container } = render(<QrCode value={PAYLOAD} label="Scan to pair" size="sm" />);
    const svg = container.querySelector<SVGSVGElement>('[data-slot="qr-code"]');
    expect(svg?.dataset.size).toBe("sm");
  });
});
