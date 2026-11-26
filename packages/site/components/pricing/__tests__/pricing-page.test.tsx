import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PricingPageContent } from "@/components/pricing/pricing-page";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: { href: string; children: ReactNode } & Omit<
    AnchorHTMLAttributes<HTMLAnchorElement>,
    "href"
  >) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

afterEach(() => cleanup());

describe("PricingPageContent", () => {
  it("defaults to annual billing, shows the tax note, and links Get Pro to cloud upgrade", () => {
    render(<PricingPageContent />);

    expect(screen.getByRole("button", { name: /Annual/ }).getAttribute("aria-pressed")).toBe(
      "true"
    );
    expect(screen.getByText("Save 33%")).toBeDefined();
    expect(screen.getByText("USD 6.67")).toBeDefined();
    expect(screen.getByText("/month, billed yearly")).toBeDefined();
    expect(screen.getByText("USD 79.99/year")).toBeDefined();
    expect(
      screen.getByText("Prices in USD. Plus applicable taxes, calculated at checkout.")
    ).toBeDefined();

    const getProLinks = screen.getAllByRole("link", { name: "Get Pro" });
    expect(getProLinks.length).toBeGreaterThan(0);
    for (const link of getProLinks) {
      expect(link.getAttribute("href")).toBe(
        "https://cloud.compozy.com/billing/upgrade?interval=year"
      );
    }

    const downloadLinks = screen.getAllByRole("link", { name: "Download" });
    for (const link of downloadLinks) {
      expect(link.getAttribute("href")).toBe("/download");
    }
  });

  it("switches to monthly billing and updates the Pro price and checkout interval", () => {
    render(<PricingPageContent />);

    fireEvent.click(screen.getByRole("button", { name: "Monthly" }));

    expect(screen.getByRole("button", { name: "Monthly" }).getAttribute("aria-pressed")).toBe(
      "true"
    );
    expect(screen.getByText("USD 9.99")).toBeDefined();
    expect(screen.getByText("/month")).toBeDefined();

    for (const link of screen.getAllByRole("link", { name: "Get Pro" })) {
      expect(link.getAttribute("href")).toBe(
        "https://cloud.compozy.com/billing/upgrade?interval=month"
      );
    }
  });

  it("renders the Free vs Pro comparison and expands FAQ answers with docs and refund links", () => {
    render(<PricingPageContent />);

    const comparison = document.getElementById("comparison");
    expect(comparison).not.toBeNull();
    expect(
      within(comparison as HTMLElement).getByText("Hosted route from any network")
    ).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: /Why does CompozyOS ask me to sign in/ }));
    expect(
      screen.getByRole("link", { name: /Why CompozyOS asks you to sign in/ }).getAttribute("href")
    ).toBe("/docs/account/sign-in");

    fireEvent.click(screen.getByRole("button", { name: /What is the refund policy/ }));
    expect(screen.getByRole("link", { name: /Read the refund policy/ }).getAttribute("href")).toBe(
      "/legal/refunds"
    );
  });
});
