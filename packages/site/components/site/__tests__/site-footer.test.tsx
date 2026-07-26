import { render, screen, within } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { footerColumns } from "@/lib/footer-config";
import { siteConfig } from "@/lib/site-config";
import { SiteFooter } from "../site-footer";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: ReactNode;
  } & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

describe("SiteFooter", () => {
  it("renders the public site identity and release footer copy", () => {
    render(<SiteFooter />);

    const footer = screen.getByRole("contentinfo");
    const homeLink = screen.getByRole("link", { name: "AGH home" });
    const logo = homeLink.querySelector('[data-slot="logo"]');

    expect(footer).toBeDefined();
    expect(screen.getByText(siteConfig.description)).toBeDefined();
    expect(homeLink.getAttribute("href")).toBe("/");
    expect(logo).not.toBeNull();
    expect(logo?.getAttribute("data-variant")).toBe("logo");
    expect(logo?.getAttribute("aria-hidden")).toBe("true");
    expect(
      screen.getByText(`© ${new Date().getFullYear()} ${siteConfig.name} · Built by Compozy.`)
    ).toBeDefined();
    expect(screen.getByRole("link", { name: "Alpha · Compozy on GitHub" })).toBeDefined();
  }, 15_000);

  it("renders every configured footer column as accessible navigation", () => {
    render(<SiteFooter />);

    for (const column of footerColumns) {
      const navigation = screen.getByRole("navigation", { name: column.title });
      const columnScreen = within(navigation);

      for (const item of column.items) {
        const link = columnScreen.getByRole("link", { name: item.label });
        expect(link.getAttribute("href")).toBe(item.href);
      }
    }
  });

  it("keeps public GitHub links explicit, external, and opener-safe", () => {
    render(<SiteFooter />);

    const githubLinks = screen
      .getAllByRole("link")
      .filter(link => link.getAttribute("href") === siteConfig.githubUrl);

    expect(githubLinks.length).toBeGreaterThanOrEqual(2);

    for (const link of githubLinks) {
      expect(link.getAttribute("target")).toBe("_blank");
      expect(link.getAttribute("rel")).toContain("noopener");
      expect(link.getAttribute("rel")).toContain("noreferrer");
    }

    expect(screen.getByRole("link", { name: "AGH on GitHub" })).toBeDefined();
  });
});
