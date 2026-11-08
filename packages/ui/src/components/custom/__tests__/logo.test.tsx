import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Logo } from "../logo";
import { LOGO_SYMBOL_PNG_256 } from "../logo-symbol.generated";
import { LOGO_LOCKUP, LOGO_WORDMARK } from "../logo-wordmark.generated";

const RETIRED_ACCENT = /#e8572a/i;

describe("Logo", () => {
  it("renders the full logo variant by default", () => {
    render(<Logo data-testid="logo" />);

    const logo = screen.getByRole("img", { name: "Compozy" });
    expect(logo).toHaveAttribute("data-slot", "logo");
    expect(logo).toHaveAttribute("data-variant", "logo");
    expect(logo).toHaveAttribute("viewBox", LOGO_LOCKUP.viewBox);
    expect(logo.querySelector("image")).toHaveAttribute("href", LOGO_SYMBOL_PNG_256);
    const wordmark = logo.querySelector(
      `g[transform="translate(${LOGO_LOCKUP.wordmark.x} ${LOGO_LOCKUP.wordmark.y})"] path`
    );
    expect(wordmark).toHaveAttribute("d", LOGO_WORDMARK.d);
    expect(logo.outerHTML).not.toMatch(RETIRED_ACCENT);
  });

  it("renders the symbol variant as the generated raster symbol", () => {
    render(<Logo variant="symbol" label="Compozy symbol" />);

    const logo = screen.getByRole("img", { name: "Compozy symbol" });
    expect(logo).toHaveAttribute("data-variant", "symbol");
    expect(logo).toHaveAttribute("viewBox", "0 0 256 256");
    const image = logo.querySelector("image");
    expect(image?.getAttribute("href")).toMatch(/^data:image\/png;base64,/);
    expect(image).toHaveAttribute("href", LOGO_SYMBOL_PNG_256);
    expect(logo.querySelector("path")).toBeNull();
  });

  it("renders the lettering variant as the vector wordmark", () => {
    render(<Logo variant="lettering" label="Compozy lettering" />);

    const logo = screen.getByRole("img", { name: "Compozy lettering" });
    expect(logo).toHaveAttribute("data-variant", "lettering");
    expect(logo).toHaveAttribute("viewBox", LOGO_WORDMARK.viewBox);
    const paths = logo.querySelectorAll("path");
    expect(paths).toHaveLength(1);
    expect(paths[0]).toHaveAttribute("fill", "currentColor");
    expect(paths[0]).toHaveAttribute("d", LOGO_WORDMARK.d);
    expect(logo.outerHTML).not.toMatch(RETIRED_ACCENT);
    expect(logo.querySelector("image")).toBeNull();
    expect(logo.querySelector("text")).toBeNull();
  });

  it("uses an explicit aria-label when provided", () => {
    render(<Logo aria-label="Compozy home" />);

    expect(screen.getByRole("img", { name: "Compozy home" })).toBeInTheDocument();
  });

  it("can render as decorative artwork", () => {
    render(<Logo decorative data-testid="logo" />);

    const logo = screen.getByTestId("logo");
    expect(screen.queryByRole("img")).toBeNull();
    expect(logo).toHaveAttribute("aria-hidden", "true");
    expect(logo).not.toHaveAttribute("aria-label");
  });

  it("passes className and style through to the svg", () => {
    render(<Logo className="size-7 opacity-80" style={{ opacity: 0.5 }} data-testid="logo" />);

    const logo = screen.getByTestId("logo");
    expect(logo.getAttribute("class")).toContain("size-7");
    expect(logo).toHaveStyle({ opacity: "0.5" });
  });
});
