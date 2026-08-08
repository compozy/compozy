// Suite: QR code rendering contract
// Invariant: QrCode preserves its accessible image semantics and canonical matrix geometry.
// Boundary IN: the shared QrCode component and its rendered SVG.
// Boundary OUT: pairing flows and scanner interoperability, owned by gateway integration QA.
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
    expect(svg).toHaveClass("size-qr-code-sm");
  });

  it("Should keep accessibility and matrix geometry invariant when SVG props conflict", () => {
    const { container, rerender } = render(
      <QrCode
        label="Scan to pair"
        role="presentation"
        shapeRendering="auto"
        value={PAYLOAD}
        viewBox="0 0 1 1"
      />
    );
    const expected = encode(PAYLOAD, { border: 4, ecc: "M" });
    const svg = container.querySelector<SVGSVGElement>('[data-slot="qr-code"]');

    expect(svg).toHaveAttribute("role", "img");
    expect(svg).toHaveAttribute("shape-rendering", "crispEdges");
    expect(svg).toHaveAttribute("viewBox", `0 0 ${expected.size} ${expected.size}`);

    rerender(<QrCode aria-label="Wrong label" label="Scan to pair" value={PAYLOAD} />);
    expect(svg).toHaveAttribute("aria-label", "Scan to pair");
  });
});
