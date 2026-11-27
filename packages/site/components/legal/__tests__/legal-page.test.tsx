import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LegalPage } from "@/components/legal/legal-page";

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

describe("LegalPage", () => {
  it("renders Terms of Service with legal tabs, last updated date, and mobile TOC toggle", () => {
    render(<LegalPage docId="terms" />);

    expect(screen.getByRole("heading", { level: 1, name: "Terms of Service" })).toBeDefined();
    expect(screen.getByText("Last updated: November 23, 2026")).toBeDefined();
    expect(screen.queryByText("Draft for legal review")).toBeNull();

    const docNav = screen.getByRole("navigation", { name: "Legal documents" });
    expect(within(docNav).getByRole("link", { name: "Terms" }).getAttribute("aria-current")).toBe(
      "page"
    );
    expect(within(docNav).getByRole("link", { name: "Privacy" }).getAttribute("href")).toBe(
      "/legal/privacy"
    );
    expect(within(docNav).getByRole("link", { name: "Refunds" }).getAttribute("href")).toBe(
      "/legal/refunds"
    );

    const tocButton = screen.getByRole("button", { name: /On this page/ });
    expect(tocButton.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(tocButton);
    expect(tocButton.getAttribute("aria-expanded")).toBe("true");
  });

  it("renders Privacy Policy with all 8 sub-processors and the hosted-route encryption note", () => {
    render(<LegalPage docId="privacy" />);

    expect(screen.getByRole("heading", { level: 1, name: "Privacy Policy" })).toBeDefined();
    const table = screen.getByRole("table");
    for (const processor of [
      "Cloudflare",
      "Neon",
      "Stripe",
      "Resend",
      "Google",
      "GitHub",
      "Sentry",
      "Better Stack",
    ]) {
      expect(within(table).getByText(processor)).toBeDefined();
    }
    expect(screen.getByText(/encrypted end to end/)).toBeDefined();
    expect(screen.getByText(/analytics scripts are never loaded/)).toBeDefined();
  });

  it("renders Refund Policy with the 14-day refund window and account-deletion rules", () => {
    render(<LegalPage docId="refunds" />);

    expect(screen.getByRole("heading", { level: 1, name: "Refund Policy" })).toBeDefined();
    expect(screen.getByText(/full refund on request within 14 days/)).toBeDefined();
    expect(screen.getByText(/An active Pro subscription never blocks you/)).toBeDefined();
  });
});
